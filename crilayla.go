package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// DecompressCRILAYLA returns decompressed data
// Based on https://github.com/esperknight/CriPakTools/blob/master/CriPakTools/CPK.cs
func DecompressCRILAYLA(input []byte) ([]byte, error) {
	reader := bytes.NewReader(input)

	const decompressedHeaderSize = 0x100

	// CRILAYLA check
	var magic [8]byte
	binary.Read(reader, binary.LittleEndian, &magic)

	if string(magic[:]) != "CRILAYLA" {
		return nil, fmt.Errorf("CRILAYLA Header not found")
	}

	var decompressedSize uint32
	binary.Read(reader, binary.LittleEndian, &decompressedSize)

	var decompressedHeaderOffset uint32
	binary.Read(reader, binary.LittleEndian, &decompressedHeaderOffset)

	// TODO error checks (out of range etc)

	result := make([]byte, decompressedSize + decompressedHeaderSize)

	// Copy uncompressed 0x100 header to start of file
	copy(result, input[decompressedHeaderOffset+0x10:decompressedHeaderOffset+decompressedHeaderSize])

	inputEnd := len(input) - decompressedHeaderSize - 1
	inputOffset := inputEnd
	outputEnd := decompressedHeaderSize + decompressedSize - 1
	bitPool := byte(0)
	bitsLeft := 0
	bytesOutput := uint32(0)
	vleLens := [4]int{2, 3, 5, 8}

	for bytesOutput < decompressedSize {

		if decompressedSize - bytesOutput < 22 {
			// TODO fix last few bytes on some files (could just be junk data)
			//fmt.Printf("bailing!")
			//break
		}

		check := getNextBits(&input, &inputOffset, &bitPool, &bitsLeft, 1)
		//fmt.Printf("offset: %d check: %d\n", inputOffset, check)

		if check > 0 {

			backreferenceOffset := outputEnd - bytesOutput + uint32(getNextBits(&input, &inputOffset, &bitPool, &bitsLeft, 13)) + 3
			backReferenceLength := 3
			vleLevel := 0

			//fmt.Printf("vle 1st\n")
			for vleLevel = 0; vleLevel < len(vleLens); vleLevel++ {
				thisLevel := getNextBits(&input, &inputOffset, &bitPool, &bitsLeft, vleLens[vleLevel])
				//fmt.Printf("this_level: %d\n", thisLevel)
				backReferenceLength += int(thisLevel)
				if thisLevel != (1 << vleLens[vleLevel] - 1) {
					break
				}
			}
			//fmt.Printf("backreference_length: %d\n", backReferenceLength)

			//fmt.Printf("vleLevel: %d\n", vleLevel)
			if vleLevel == len(vleLens) {
				var thisLevel int

				for ok := true; ok; ok = thisLevel == 255 {
					thisLevel = int(getNextBits(&input, &inputOffset, &bitPool, &bitsLeft, 8))
					//fmt.Printf("vle 2nd this_level: %d\n", thisLevel)
					backReferenceLength += thisLevel
				}
				//fmt.Printf("backreference_length: %d\n", backReferenceLength)
			}


			for i := 0; i < backReferenceLength; i++ {
				result[outputEnd - bytesOutput] = result[backreferenceOffset]
				backreferenceOffset--
				bytesOutput++
			}
			//fmt.Printf("bytes_output: %d\n", bytesOutput)
		} else {
			// verbatim byte
			result[outputEnd - bytesOutput] = byte(getNextBits(&input, &inputOffset, &bitPool, &bitsLeft, 8))
			bytesOutput++
			//fmt.Printf("read_byte bytes_output: %d\n", bytesOutput)
		}
	}

	return result, nil
}

func getNextBits(input *[]byte, offset *int, bitPool *byte, bitsLeft *int, bitCount int) uint16 {
	outBits := uint16(0)
	numBitsProduced := 0
	var bitsThisRound int


	for numBitsProduced < bitCount {

		if *bitsLeft == 0 {
			*bitPool = (*input)[*offset]
			*bitsLeft = 8
			*offset--
		}

		if *bitsLeft > (bitCount - numBitsProduced) {
			bitsThisRound = bitCount - numBitsProduced
		} else {
			bitsThisRound = *bitsLeft
		}

		outBits <<= bitsThisRound

		outBits |= uint16(uint16(*bitPool>> (*bitsLeft- bitsThisRound)) & ((1 << bitsThisRound) - 1))

		*bitsLeft -= bitsThisRound
		numBitsProduced += bitsThisRound
	}

	//fmt.Printf("getNextBits: %d\n", outBits)
	return outBits
}
