//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
package knxnetip

import (
    "errors"
    "github.com/apache/plc4x/plc4go/internal/plc4go/knxnetip/readwrite"
    driverModel "github.com/apache/plc4x/plc4go/internal/plc4go/knxnetip/readwrite/model"
    "github.com/apache/plc4x/plc4go/internal/plc4go/spi"
    internalModel "github.com/apache/plc4x/plc4go/internal/plc4go/spi/model"
    "github.com/apache/plc4x/plc4go/internal/plc4go/spi/utils"
    internalValues "github.com/apache/plc4x/plc4go/internal/plc4go/spi/values"
    apiModel "github.com/apache/plc4x/plc4go/pkg/plc4go/model"
    apiValues "github.com/apache/plc4x/plc4go/pkg/plc4go/values"
    "strconv"
    "time"
)

type KnxNetIpReader struct {
    connection *KnxNetIpConnection
    spi.PlcReader
}

func NewKnxNetIpReader(connection *KnxNetIpConnection) *KnxNetIpReader {
    return &KnxNetIpReader{
        connection: connection,
    }
}

func (m KnxNetIpReader) Read(readRequest apiModel.PlcReadRequest) <-chan apiModel.PlcReadRequestResult {
    resultChan := make(chan apiModel.PlcReadRequestResult)
    go func() {
        responseCodes := map[string]apiModel.PlcResponseCode{}
        plcValues := map[string]apiValues.PlcValue{}

        // Sort the fields in direct properties, which will have to be actively read from the devices
        // and group-addresses which will be locally processed from the local cache.
        directProperties := map[driverModel.KnxAddress]map[string]KnxNetIpDevicePropertyAddressPlcField{}
        groupAddresses := map[string]KnxNetIpField{}
        for _, fieldName := range readRequest.GetFieldNames() {
            // Get the knx field
            field, err := CastToKnxNetIpFieldFromPlcField(readRequest.GetField(fieldName))
            if err != nil {
                responseCodes[fieldName] = apiModel.PlcResponseCode_INVALID_ADDRESS
                plcValues[fieldName] = nil
                continue
            }

            switch field.(type) {
            case KnxNetIpDevicePropertyAddressPlcField:
                propertyField := field.(KnxNetIpDevicePropertyAddressPlcField)
                knxAddress := FieldToKnxAddress(propertyField)
                if knxAddress == nil {
                    continue
                }
                if _, ok := directProperties[*knxAddress]; !ok {
                    directProperties[*knxAddress] = map[string]KnxNetIpDevicePropertyAddressPlcField{}
                }
                directProperties[*knxAddress][fieldName] = propertyField
            default:
                groupAddresses[fieldName] = field
            }
        }

        // Process the direct properties.
        // Connect to each knx device and read all of the properties on that particular device.
        // Finish up by explicitly disconnecting after all properties on the device have been read.
        for deviceAddress, fields := range directProperties {
            // Connect to the device
            err := m.connectToDevice(deviceAddress)
            // If something went wrong all field for this device are equally failed
            if err != nil {
                for fieldName := range fields {
                    responseCodes[fieldName] = apiModel.PlcResponseCode_INVALID_ADDRESS
                    plcValues[fieldName] = nil
                }
                continue
            }

            // Collect all the properties on this device
            counter := uint8(1)
            for fieldName, field := range fields {
                responseCode, plcValue := m.readDeviceProperty(field, counter)
                responseCodes[fieldName] = responseCode
                plcValues[fieldName] = plcValue
                counter++
            }

            // Disconnect from the device
            _ = m.disconnectFromDevice(*m.connection.ClientKnxAddress, deviceAddress)
            // In this case we ignore if something goes wrong
        }

        // Get the group address values from the cache
        for fieldName, field := range groupAddresses {
            responseCode, plcValue := m.readGroupAddress(field)
            responseCodes[fieldName] = responseCode
            plcValues[fieldName] = plcValue
        }

        // Assemble the results
        result := internalModel.NewDefaultPlcReadResponse(readRequest, responseCodes, plcValues)
        resultChan <- apiModel.PlcReadRequestResult{
            Request:  readRequest,
            Response: result,
            Err:      nil,
        }
    }()
    return resultChan
}

func (m KnxNetIpReader) connectToDevice(targetAddress driverModel.KnxAddress) error {
    connectionSuccess := make(chan bool)
    deviceConnectionRequest := driverModel.NewTunnelingRequest(
        driverModel.NewTunnelingRequestDataBlock(0, 0),
        driverModel.NewLDataReq(0, nil,
            driverModel.NewLDataFrameDataExt(false, 6, 0,
                driverModel.NewKnxAddress(0, 0, 0), KnxAddressToInt8Array(targetAddress),
                driverModel.NewApduControlContainer(driverModel.NewApduControlConnect(), 0, false, 0),
                true, true, driverModel.CEMIPriority_SYSTEM, false, false)))

    // Send the request
    err := m.connection.SendRequest(
        deviceConnectionRequest,
        // The Gateway is now supposed to send an Ack to this request.
        func(message interface{}) bool {
            tunnelingRequest := driverModel.CastTunnelingRequest(message)
            if tunnelingRequest == nil ||
                tunnelingRequest.TunnelingRequestDataBlock.CommunicationChannelId != m.connection.CommunicationChannelId {
                return false
            }
            lDataCon := driverModel.CastLDataCon(tunnelingRequest.Cemi)
            return lDataCon != nil
        },
        func(message interface{}) error {
            tunnelingRequest := driverModel.CastTunnelingRequest(message)
            lDataCon := driverModel.CastLDataCon(tunnelingRequest.Cemi)
            // If the error flag is set, there was an error connecting
            if lDataCon.DataFrame.ErrorFlag {
                connectionSuccess <- false
            }

            // Now for some reason it seems as if we need to implement a Device Descriptor read.
            deviceDescriptorReadRequest := driverModel.NewTunnelingRequest(
                driverModel.NewTunnelingRequestDataBlock(0, 0),
                driverModel.NewLDataReq(0, nil,
                    driverModel.NewLDataFrameDataExt(false, 6, uint8(0),
                        driverModel.NewKnxAddress(0, 0, 0), KnxAddressToInt8Array(targetAddress),
                        driverModel.NewApduDataContainer(driverModel.NewApduDataDeviceDescriptorRead(0), 1, true, 0),
                        true, true, driverModel.CEMIPriority_LOW, false, false)))
            _ = m.connection.SendRequest(
                deviceDescriptorReadRequest,
                func(message interface{}) bool {
                    tunnelingRequest := driverModel.CastTunnelingRequest(message)
                    if tunnelingRequest == nil ||
                        tunnelingRequest.TunnelingRequestDataBlock.CommunicationChannelId != m.connection.CommunicationChannelId {
                        return false
                    }
                    lDataInd := driverModel.CastLDataInd(tunnelingRequest.Cemi)
                    if lDataInd == nil {
                        return false
                    }
                    dataFrame := driverModel.CastLDataFrameDataExt(lDataInd.DataFrame)
                    if dataFrame == nil {
                        return false
                    }
                    dataContainer := driverModel.CastApduDataContainer(dataFrame.Apdu)
                    if dataContainer == nil {
                        return false
                    }
                    deviceDescriptorResponse := driverModel.CastApduDataDeviceDescriptorResponse(dataContainer.DataApdu)
                    if deviceDescriptorResponse == nil {
                        return false
                    }
                    return true
                    //return *dataFrame.Apdu.Apci == driverModel.APCI_DEVICE_DESCRIPTOR_RESPONSE_PDU
                    // TODO: Do something with the request ...
                },
                func(message interface{}) error {
                    // Send back an ACK
                    _ = m.connection.Send(
                        driverModel.NewTunnelingRequest(
                            driverModel.NewTunnelingRequestDataBlock(0, 0),
                            driverModel.NewLDataReq(0, nil,
                                driverModel.NewLDataFrameDataExt(false, 6, uint8(0),
                                    driverModel.NewKnxAddress(0, 0, 0), KnxAddressToInt8Array(targetAddress),
                                    driverModel.NewApduControlContainer(driverModel.NewApduControlAck(), 0, true, 0),
                                    true, true, driverModel.CEMIPriority_SYSTEM, false, false))))
                    // Now we can finally read properties.
                    connectionSuccess <- true
                    return nil
                },
                time.Second*5)
            return nil
        },
        time.Second*1)

    if err != nil {
        return errors.New("could not connect to device (Error sending connection request)")
    }
    select {
    case result := <-connectionSuccess:
        if !result {
            return errors.New("could not connect to device (NACK)")
        }
    case <-time.After(time.Second * 5):
        return errors.New("could not connect to device (Timeout)")
    }
    return nil
}

func (m KnxNetIpReader) disconnectFromDevice(sourceAddress driverModel.KnxAddress, targetAddress driverModel.KnxAddress) error {
    deviceConnectionRequest := driverModel.NewTunnelingRequest(
        driverModel.NewTunnelingRequestDataBlock(0, 0),
        driverModel.NewLDataReq(0, nil,
            driverModel.NewLDataFrameDataExt(false, 6, uint8(0),
                &sourceAddress, KnxAddressToInt8Array(targetAddress),
                driverModel.NewApduControlContainer(driverModel.NewApduControlDisconnect(), 0, false, 0),
                true, true, driverModel.CEMIPriority_SYSTEM, false, false)))

    // Send the request
    connectionSuccess := make(chan bool)
    err := m.connection.SendRequest(
        deviceConnectionRequest,
        // The Gateway is now supposed to send an Ack to this request.
        func(message interface{}) bool {
            tunnelingRequest := driverModel.CastTunnelingRequest(message)
            if tunnelingRequest == nil ||
                tunnelingRequest.TunnelingRequestDataBlock.CommunicationChannelId != m.connection.CommunicationChannelId {
                return false
            }
            lDataCon := driverModel.CastLDataCon(tunnelingRequest.Cemi)
            if lDataCon == nil {
                return false
            }
            frameDataExt := driverModel.CastLDataFrameDataExt(lDataCon.DataFrame)
            if frameDataExt == nil {
                return false
            }
            return true
            //return frameDataExt.Control == true && frameDataExt.ControlType == nil
        },
        func(message interface{}) error {
            tunnelingRequest := driverModel.CastTunnelingRequest(message)
            lDataCon := driverModel.CastLDataCon(tunnelingRequest.Cemi)
            _ = driverModel.CastLDataFrameDataExt(lDataCon.DataFrame)
            //if *frameDataExt.ControlType == driverModel.ControlType_DISCONNECT {
            connectionSuccess <- false
            //}
            return nil
        },
        time.Second*1)

    if err != nil {
        return errors.New("could not connect to device (Error sending connection request)")
    }
    select {
    case result := <-connectionSuccess:
        if !result {
            return errors.New("could not connect to device (NACK)")
        }
    case <-time.After(time.Second * 5):
        return errors.New("could not connect to device (Timeout)")
    }
    return nil
}

type readPropertyResult struct {
    plcValue     apiValues.PlcValue
    responseCode apiModel.PlcResponseCode
}

func (m KnxNetIpReader) readDeviceProperty(field KnxNetIpDevicePropertyAddressPlcField, counter uint8) (apiModel.PlcResponseCode, apiValues.PlcValue) {
    // TODO: We'll add this as time progresses, for now we only support fully qualified addresses
    if field.IsPatternField() {
        return apiModel.PlcResponseCode_UNSUPPORTED, nil
    }

    destinationAddress := FieldToKnxAddress(field)
    destinationAddressBuffer := utils.NewWriteBuffer()
    err := destinationAddress.Serialize(*destinationAddressBuffer)
    if err != nil {
        return apiModel.PlcResponseCode_INTERNAL_ERROR, nil
    }
    destinationAddressData := utils.Uint8ArrayToInt8Array(destinationAddressBuffer.GetBytes())
    objectId, _ := strconv.Atoi(field.ObjectId)
    propertyId, _ := strconv.Atoi(field.PropertyId)

    request := driverModel.NewTunnelingRequest(
        driverModel.NewTunnelingRequestDataBlock(0, 0),
        driverModel.NewLDataReq(0, nil,
            driverModel.NewLDataFrameDataExt(false, 6, 0,
                driverModel.NewKnxAddress(0, 0, 0),
                destinationAddressData,
                driverModel.NewApduDataContainer(
                    driverModel.NewApduDataOther(
                        // TODO: The counter should be incremented per KNX individual address
                        driverModel.NewApduDataExtPropertyValueRead(uint8(objectId), uint8(propertyId), 1, 1)), 5, true, counter),
                true, true, driverModel.CEMIPriority_LOW, false, false)))

    result := make(chan readPropertyResult)
    err = m.connection.SendRequest(
        request,
        // Even if there are multiple messages being exchanged because of the request
        // We are not interested in most of them. The one containing the response is
        // an LData.ind from the destination address to our client address with the given
        // object-id and property-id.
        func(message interface{}) bool {
            tunnelingRequest := driverModel.CastTunnelingRequest(message)
            if tunnelingRequest == nil {
                return false
            }
            lDataInd := driverModel.CastLDataInd(tunnelingRequest.Cemi)
            if lDataInd == nil {
                return false
            }
            dataFrameExt := driverModel.CastLDataFrameDataExt(lDataInd.DataFrame)
            if dataFrameExt != nil && dataFrameExt.Apdu != nil {
                apduDataContainer := driverModel.CastApduDataContainer(dataFrameExt.Apdu)
                if apduDataContainer == nil {
                    return false
                }
                otherPdu := driverModel.CastApduDataOther(apduDataContainer.DataApdu)
                if otherPdu == nil {
                    return false
                }
                propertyValueResponse := driverModel.CastApduDataExtPropertyValueResponse(otherPdu.ExtendedApdu)
                if propertyValueResponse == nil {
                    return false
                }
                if *dataFrameExt.SourceAddress != *destinationAddress {
                    return false
                }
                if *Int8ArrayToKnxAddress(dataFrameExt.DestinationAddress) != *m.connection.ClientKnxAddress {
                    return false
                }
                curObjectId := propertyValueResponse.ObjectIndex
                curPropertyId := propertyValueResponse.PropertyId
                if curObjectId == uint8(objectId) && curPropertyId == uint8(propertyId) {
                    return true
                }
            }
            return false
        },
        func(message interface{}) error {
            tunnelingRequest := driverModel.CastTunnelingRequest(message)
            lDataInd := driverModel.CastLDataInd(tunnelingRequest.Cemi)
            dataFrameExt := driverModel.CastLDataFrameDataExt(lDataInd.DataFrame)
            apduDataContainer := driverModel.CastApduDataContainer(dataFrameExt.Apdu)
            otherPdu := driverModel.CastApduDataOther(apduDataContainer.DataApdu)
            propertyValueResponse := driverModel.CastApduDataExtPropertyValueResponse(otherPdu.ExtendedApdu)

            readBuffer := utils.NewReadBuffer(propertyValueResponse.Data)
            propertyCount := propertyValueResponse.Count

            // If the return is a count of 0, then we can't access this property (Doesn't exist or not allowed to)
            // As we don't have a return code for "doesn't exist or doesn't have access to" we'll stick to "not found"
            // as this can be understood as "found no property we have access to"
            // ("03_03_07 Application Layer v01.06.02 AS" Page 52)
            var propResult readPropertyResult
            if propertyCount == 0 {
                propResult = readPropertyResult{
                    responseCode: apiModel.PlcResponseCode_NOT_FOUND,
                }
            } else {
                // Depending on the object id and property id, parse the remaining data accordingly.
                property := driverModel.KnxInterfaceObjectProperty_PID_UNKNOWN
                for i := driverModel.KnxInterfaceObjectProperty_PID_UNKNOWN; i < driverModel.KnxInterfaceObjectProperty_PID_SUNBLIND_SENSOR_BASIC_ENABLE_TOGGLE_MODE; i++ {
                    // If the propertyId matches and this is either a general object or the object id matches, add it to the result
                    if i.PropertyId() == uint8(propertyId) && (i.ObjectType().Code() == "G" || i.ObjectType().Code() == strconv.Itoa(objectId)) {
                        property = i
                        break
                    }
                }

                // Parse the payload according to the specified datatype
                dataType := property.PropertyDataType()
                plcValue := readwrite.ParsePropertyDataType(*readBuffer, dataType, dataType.SizeInBytes())
                propResult = readPropertyResult{
                    plcValue:     plcValue,
                    responseCode: apiModel.PlcResponseCode_OK,
                }
            }

            // Send back an ACK
            _ = m.connection.Send(
                driverModel.NewTunnelingRequest(
                    driverModel.NewTunnelingRequestDataBlock(0, 0),
                    driverModel.NewLDataReq(0, nil,
                        driverModel.NewLDataFrameDataExt(false, 6, uint8(0),
                            driverModel.NewKnxAddress(0, 0, 0), destinationAddressData,
                            driverModel.NewApduControlContainer(driverModel.NewApduControlAck(), 0, true, dataFrameExt.Apdu.Counter),
                            true, true, driverModel.CEMIPriority_SYSTEM, false, false))))

            result <- propResult
            return nil
        },
        time.Second*5)

    select {
    case value := <-result:
        responseCode := value.responseCode
        plcValue := value.plcValue
        return responseCode, plcValue
        /*case <-time.After(time.Second * 5):
          return apiModel.PlcResponseCode_REMOTE_ERROR, nil*/
    }
}

func (m KnxNetIpReader) readGroupAddress(field KnxNetIpField) (apiModel.PlcResponseCode, apiValues.PlcValue) {
    // Pattern fields can match more than one value, therefore we have to handle things differently
    if field.IsPatternField() {
        // Depending on the type of field, get the uint16 ids of all values that match the current field
        matchedAddresses := map[uint16]*driverModel.KnxGroupAddress{}
        switch field.(type) {
        case KnxNetIpGroupAddress3LevelPlcField:
            for key, value := range m.connection.leve3AddressCache {
                if field.matches(value.Parent) {
                    matchedAddresses[key] = value.Parent
                }
            }
        case KnxNetIpGroupAddress2LevelPlcField:
            for key, value := range m.connection.leve2AddressCache {
                if field.matches(value.Parent) {
                    matchedAddresses[key] = value.Parent
                }
            }
        case KnxNetIpGroupAddress1LevelPlcField:
            for key, value := range m.connection.leve1AddressCache {
                if field.matches(value.Parent) {
                    matchedAddresses[key] = value.Parent
                }
            }
        }

        // If not a single match was found, we'll return a "not found" message
        if len(matchedAddresses) == 0 {
            return apiModel.PlcResponseCode_NOT_FOUND, nil
        }

        // Go through all of the values and create a plc-struct from them
        // where the string version of the address becomes the property name
        // and the property value is the corresponding value (Other wise it
        // would be impossible to know which of the fields the pattern matched
        // a given value belongs to)
        values := map[string]apiValues.PlcValue{}
        for numericAddress, address := range matchedAddresses {
            // Get the raw data from the cache
            m.connection.valueCacheMutex.RLock()
            int8s, _ := m.connection.valueCache[numericAddress]
            m.connection.valueCacheMutex.RUnlock()

            // If we don't have any field-type information, add the raw data
            if field.GetTypeName() == "" {
                values[GroupAddressToString(address)] =
                    internalValues.NewPlcByteArray(utils.Int8ArrayToByteArray(int8s))
            } else {
                // Decode the data according to the fields type
                rb := utils.NewReadBuffer(utils.Int8ArrayToUint8Array(int8s))
                if field.GetFieldType() == nil {
                    return apiModel.PlcResponseCode_INVALID_DATATYPE, nil
                }
                plcValue, err := driverModel.KnxDatapointParse(rb, *field.GetFieldType())
                // If any of the values doesn't decode correctly, we can't return any
                if err != nil {
                    return apiModel.PlcResponseCode_INVALID_DATA, nil
                }
                values[GroupAddressToString(address)] = plcValue
            }
        }

        // Add it to the result
        return apiModel.PlcResponseCode_OK, internalValues.NewPlcStruct(values)
    } else {
        // If it's not a pattern field, we can access the cached value a lot simpler

        // Serialize the field to an uint16
        wb := utils.NewWriteBuffer()
        err := field.toGroupAddress().Serialize(*wb)
        if err != nil {
            return apiModel.PlcResponseCode_INVALID_ADDRESS, nil
        }
        rawAddress := wb.GetBytes()
        address := (uint16(rawAddress[0]) << 8) | uint16(rawAddress[1]&0xFF)

        // Get the value form the cache
        m.connection.valueCacheMutex.RLock()
        int8s, ok := m.connection.valueCache[address]
        m.connection.valueCacheMutex.RUnlock()
        if !ok {
            return apiModel.PlcResponseCode_NOT_FOUND, nil
        }

        // Decode the data according to the fields type
        rb := utils.NewReadBuffer(utils.Int8ArrayToUint8Array(int8s))
        if field.GetFieldType() == nil {
            return apiModel.PlcResponseCode_INVALID_DATATYPE, nil
        }
        plcValue, err := driverModel.KnxDatapointParse(rb, *field.GetFieldType())
        if err != nil {
            return apiModel.PlcResponseCode_INVALID_DATA, nil
        }

        // Add it to the result
        return apiModel.PlcResponseCode_OK, plcValue
    }
}
