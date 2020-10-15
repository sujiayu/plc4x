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

#include <stdio.h>
#include <plc4c/spi/evaluation_helper.h>
#include "s7_address.h"

// Array of discriminator values that match the enum type constants.
// (The order is identical to the enum constants so we can use the
// enum constant to directly access a given types discriminator values)
const plc4c_s7_read_write_s7_address_discriminator plc4c_s7_read_write_s7_address_discriminators[] = {
  {/* plc4c_s7_read_write_s7_address_any */
   .addressType = 0x10}
};

// Function returning the discriminator values for a given type constant.
plc4c_s7_read_write_s7_address_discriminator plc4c_s7_read_write_s7_address_get_discriminator(plc4c_s7_read_write_s7_address_type type) {
  return plc4c_s7_read_write_s7_address_discriminators[type];
}

// Create an empty NULL-struct
static const plc4c_s7_read_write_s7_address plc4c_s7_read_write_s7_address_null_const;

plc4c_s7_read_write_s7_address plc4c_s7_read_write_s7_address_null() {
  return plc4c_s7_read_write_s7_address_null_const;
}


// Parse function.
plc4c_return_code plc4c_s7_read_write_s7_address_parse(plc4c_spi_read_buffer* io, plc4c_s7_read_write_s7_address** _message) {
  uint16_t startPos = plc4c_spi_read_get_pos(io);
  uint16_t curPos;
  plc4c_return_code _res = OK;

  // Allocate enough memory to contain this data structure.
  (*_message) = malloc(sizeof(plc4c_s7_read_write_s7_address));
  if(*_message == NULL) {
    return NO_MEMORY;
  }

  // Discriminator Field (addressType) (Used as input to a switch field)
  uint8_t addressType = 0;
  _res = plc4c_spi_read_unsigned_byte(io, 8, (uint8_t*) &addressType);
  if(_res != OK) {
    return _res;
  }

  // Switch Field (Depending on the discriminator values, passes the instantiation to a sub-type)
  if(addressType == 0x10) { /* S7AddressAny */
    (*_message)->_type = plc4c_s7_read_write_s7_address_type_plc4c_s7_read_write_s7_address_any;
                    
    // Enum field (transportSize)
    plc4c_s7_read_write_transport_size transportSize = plc4c_s7_read_write_transport_size_null();
    _res = plc4c_spi_read_signed_byte(io, 8, (int8_t*) &transportSize);
    if(_res != OK) {
      return _res;
    }
    (*_message)->s7_address_any_transport_size = transportSize;


                    
    // Simple Field (numberOfElements)
    uint16_t numberOfElements = 0;
    _res = plc4c_spi_read_unsigned_short(io, 16, (uint16_t*) &numberOfElements);
    if(_res != OK) {
      return _res;
    }
    (*_message)->s7_address_any_number_of_elements = numberOfElements;


                    
    // Simple Field (dbNumber)
    uint16_t dbNumber = 0;
    _res = plc4c_spi_read_unsigned_short(io, 16, (uint16_t*) &dbNumber);
    if(_res != OK) {
      return _res;
    }
    (*_message)->s7_address_any_db_number = dbNumber;


                    
    // Enum field (area)
    plc4c_s7_read_write_memory_area area = plc4c_s7_read_write_memory_area_null();
    _res = plc4c_spi_read_unsigned_byte(io, 8, (uint8_t*) &area);
    if(_res != OK) {
      return _res;
    }
    (*_message)->s7_address_any_area = area;


                    
    // Reserved Field (Compartmentalized so the "reserved" variable can't leak)
    {
      unsigned int _reserved = 0;
      _res = plc4c_spi_read_unsigned_byte(io, 5, (uint8_t*) &_reserved);
      if(_res != OK) {
        return _res;
      }
      if(_reserved != 0x00) {
        printf("Expected constant value '%d' but got '%d' for reserved field.", 0x00, _reserved);
      }
    }


                    
    // Simple Field (byteAddress)
    uint16_t byteAddress = 0;
    _res = plc4c_spi_read_unsigned_short(io, 16, (uint16_t*) &byteAddress);
    if(_res != OK) {
      return _res;
    }
    (*_message)->s7_address_any_byte_address = byteAddress;


                    
    // Simple Field (bitAddress)
    unsigned int bitAddress = 0;
    _res = plc4c_spi_read_unsigned_byte(io, 3, (uint8_t*) &bitAddress);
    if(_res != OK) {
      return _res;
    }
    (*_message)->s7_address_any_bit_address = bitAddress;

  }

  return OK;
}

plc4c_return_code plc4c_s7_read_write_s7_address_serialize(plc4c_spi_write_buffer* io, plc4c_s7_read_write_s7_address* _message) {
  plc4c_return_code _res = OK;

  // Discriminator Field (addressType)
  plc4c_spi_write_unsigned_byte(io, 8, plc4c_s7_read_write_s7_address_get_discriminator(_message->_type).addressType);

  // Switch Field (Depending of the current type, serialize the sub-type elements)
  switch(_message->_type) {
    case plc4c_s7_read_write_s7_address_type_plc4c_s7_read_write_s7_address_any: {

      // Enum field (transportSize)
      _res = plc4c_spi_write_signed_byte(io, 8, _message->s7_address_any_transport_size);
      if(_res != OK) {
        return _res;
      }

      // Simple Field (numberOfElements)
      _res = plc4c_spi_write_unsigned_short(io, 16, _message->s7_address_any_number_of_elements);
      if(_res != OK) {
        return _res;
      }

      // Simple Field (dbNumber)
      _res = plc4c_spi_write_unsigned_short(io, 16, _message->s7_address_any_db_number);
      if(_res != OK) {
        return _res;
      }

      // Enum field (area)
      _res = plc4c_spi_write_unsigned_byte(io, 8, _message->s7_address_any_area);
      if(_res != OK) {
        return _res;
      }

      // Reserved Field
      _res = plc4c_spi_write_unsigned_byte(io, 5, 0x00);
      if(_res != OK) {
        return _res;
      }

      // Simple Field (byteAddress)
      _res = plc4c_spi_write_unsigned_short(io, 16, _message->s7_address_any_byte_address);
      if(_res != OK) {
        return _res;
      }

      // Simple Field (bitAddress)
      _res = plc4c_spi_write_unsigned_byte(io, 3, _message->s7_address_any_bit_address);
      if(_res != OK) {
        return _res;
      }

      break;
    }
  }

  return OK;
}

uint16_t plc4c_s7_read_write_s7_address_length_in_bytes(plc4c_s7_read_write_s7_address* _message) {
  return plc4c_s7_read_write_s7_address_length_in_bits(_message) / 8;
}

uint16_t plc4c_s7_read_write_s7_address_length_in_bits(plc4c_s7_read_write_s7_address* _message) {
  uint16_t lengthInBits = 0;

  // Discriminator Field (addressType)
  lengthInBits += 8;

  // Depending of the current type, add the length of sub-type elements ...
  switch(_message->_type) {
    case plc4c_s7_read_write_s7_address_type_plc4c_s7_read_write_s7_address_any: {

      // Enum Field (transportSize)
      lengthInBits += 8;


      // Simple field (numberOfElements)
      lengthInBits += 16;


      // Simple field (dbNumber)
      lengthInBits += 16;


      // Enum Field (area)
      lengthInBits += 8;


      // Reserved Field (reserved)
      lengthInBits += 5;


      // Simple field (byteAddress)
      lengthInBits += 16;


      // Simple field (bitAddress)
      lengthInBits += 3;

      break;
    }
  }

  return lengthInBits;
}
