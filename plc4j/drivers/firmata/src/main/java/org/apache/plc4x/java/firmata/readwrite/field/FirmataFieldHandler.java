/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/
package org.apache.plc4x.java.firmata.readwrite.field;

import org.apache.plc4x.java.api.exceptions.PlcInvalidFieldException;
import org.apache.plc4x.java.api.model.PlcField;
import org.apache.plc4x.java.api.value.*;
import org.apache.plc4x.java.spi.connection.DefaultPlcFieldHandler;

import java.util.ArrayList;
import java.util.BitSet;
import java.util.LinkedList;
import java.util.List;

public class FirmataFieldHandler extends DefaultPlcFieldHandler {

    @Override
    public PlcField createField(String fieldQuery) {
        if (FirmataFieldDigital.ADDRESS_PATTERN.matcher(fieldQuery).matches()) {
            return FirmataFieldDigital.of(fieldQuery);
        } else if (FirmataFieldAnalog.ADDRESS_PATTERN.matcher(fieldQuery).matches()) {
            return FirmataFieldAnalog.of(fieldQuery);
        }
        throw new PlcInvalidFieldException(fieldQuery);
    }
}
