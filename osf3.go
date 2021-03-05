package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// Based on https://github.com/Liquid-S/OFS3-TOOL

func UnpackOFS3File(filePath string, destDir string) error {
	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	return UnpackOFS3(fileData, destDir)
}

// UnpackOFS3 does what it says on the tin
func UnpackOFS3(fileData []byte, destDir string) error {
	reader := bytes.NewReader(fileData)

	// Check for magic of OSF3
	var magic [4]byte
	binary.Read(reader, binary.LittleEndian, &magic)
	magicStr := string(magic[:])

	if magicStr != "OFS3" {
		return fmt.Errorf("wrong magic, expected OSF3, got: %s", magicStr)
	}

	// Make folder
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		err := os.Mkdir(destDir, 0755)
		if err != nil {
			return err
		}
	}

	var unknown uint32
	binary.Read(reader, binary.LittleEndian, &unknown)

	var fileType uint16
	binary.Read(reader, binary.LittleEndian, &fileType)

	var padding byte
	binary.Read(reader, binary.LittleEndian, &padding)

	// There are two Types of Type "0x00": One has SubType "0x00" (this specifies the size of each file) and the other has "0x01" (this doesn't).
	var subType byte
	binary.Read(reader, binary.LittleEndian, &subType)

	var size uint32
	binary.Read(reader, binary.LittleEndian, &size)

	var numFiles uint32
	binary.Read(reader, binary.LittleEndian, &numFiles)

	extractedFileOffset := make([]uint32, numFiles)
	extractedFileSize := make([]uint32, numFiles)
	fileNameOffset := make([]uint32, numFiles)

	// Read offset, the size and the name offset of each file.
	for i := uint32(0); i < numFiles; i++ {
		var fOffset uint32
		binary.Read(reader, binary.LittleEndian, &fOffset)
		extractedFileOffset[i] = fOffset + 0x10

		binary.Read(reader, binary.LittleEndian, &extractedFileSize[i])

		if fileType == 0x02 {
			var fNameOffset uint32
			binary.Read(reader, binary.LittleEndian, &fNameOffset)
			fileNameOffset[i] = fNameOffset + 0x10
		}
	}

	for i := uint32(0); i < numFiles; i++ {
		// Files with SubType == 1 doesn't specify the size of each file, therefore we need to calculate it.
		if subType == 1 {
			if extractedFileOffset[i] != 0 && i == numFiles - 1 {
				extractedFileSize[i] = uint32(len(fileData)) - extractedFileOffset[i]
			} else if extractedFileOffset[i] != 0 && i < numFiles -1 {
				extractedFileSize[i] = extractedFileOffset[i + 1] - extractedFileSize[i]
			}
		}

		// Reads and saves the name of each file.
		var newFileName string
		if fileType == 0x02 {
			reader.Seek(int64(fileNameOffset[i]), io.SeekStart)
			readString(reader, binary.LittleEndian, &newFileName)
			newFileName = path.Join(destDir, "(" + fmt.Sprintf("%04d", i) + ")_") + newFileName
		} else {
			newFileName = path.Join(destDir, "(" + fmt.Sprintf("%04d", i) + ")")
		}

		// Read the new file's data and save it in "NewFileBody".
		reader.Seek(int64(extractedFileOffset[i]), io.SeekStart)
		newFileBody := make([]byte, extractedFileSize[i])
		reader.Read(newFileBody)

		// Establish the file's extension.
		if len(newFileBody) > 8 {
			if !strings.Contains(newFileName, ".ofs3") && (newFileBody[0] == 0x4F && newFileBody[1] == 0x46 && newFileBody[2] == 0x53 && newFileBody[3] == 0x33) {
				newFileName += ".ofs3"
			} else if fileType == 0 && (newFileBody[0] == 0x1F && newFileBody[1] == 0x8B && newFileBody[2] == 0x08) {
				newFileName += ".gz"
			} else if fileType == 0 && (newFileBody[0] == 0x4F && newFileBody[1] == 0x4D && newFileBody[2] == 0x47 && newFileBody[3] == 0x2E && newFileBody[4] == 0x30 && newFileBody[5] == 0x30) {
				newFileName += ".gmo"
			} else if fileType == 0 && (newFileBody[0] == 0x4D && newFileBody[1] == 0x49 && newFileBody[2] == 0x47 && newFileBody[3] == 0x2E && newFileBody[4] == 0x30 && newFileBody[5] == 0x30) {
				newFileName += ".gim"
			} else if fileType == 0 && (newFileBody[0] == 0x54 && newFileBody[1] == 0x49 && newFileBody[2] == 0x4D && newFileBody[3] == 0x32) {
				newFileName += ".tm2"
			} else if fileType == 0 && (newFileBody[0] == 0x50 && newFileBody[1] == 0x49 && newFileBody[2] == 0x4D && newFileBody[3] == 0x32) {
				newFileName += ".pm2"
			}
		}

		// Save the extracted file in DestinationDir.
		err := ioutil.WriteFile(newFileName, newFileBody, 0666)
		if err != nil {
			return err
		}
		fmt.Printf("Saved file: %s\n", newFileName)

		// Recursion time! Check if the new file is a GZip or a OFS3.
		if len(newFileBody) > 4 {
			if newFileBody[0] == 0x1F && newFileBody[1] == 0x8B && newFileBody[2] == 0x08 {
				fmt.Printf("Processing gzip: %s\n", newFileName)
				err := decompressGzip(newFileName)
				if err != nil {
					return err
				}
			} else if newFileBody[0] == 0x4F && newFileBody[1] == 0x46 && newFileBody[2] == 0x53 && newFileBody[3] == 0x33 {
				fmt.Printf("Processing osf3: %s\n", newFileName)
				err := UnpackOFS3File(newFileName, path.Join(path.Dir(newFileName), "EXTRACTED_"+path.Base(newFileName)))
				if err != nil {
					return err
				}
			}
		}

	}

	return nil
}

func decompressGzip(compressedFile string) error {

	fileData, err := ioutil.ReadFile(compressedFile)
	if err != nil {
		return err
	}


	decompressedFile := compressedFile + ".decompressed"

	r, err := zlib.NewReader(bytes.NewReader(fileData))
	if err != nil {
		return err
	}
	decompressedData, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(decompressedData)

	// Check out if the decompressed file is a ".ofs3" and saves it's extension.
	var magic [4]byte
	binary.Read(reader, binary.LittleEndian, &magic)
	magicStr := string(magic[:])
	if magicStr == "OFS3" {
		decompressedFile += ".ofs3"
	}

	// Decompress the file and save it.
	err = ioutil.WriteFile(decompressedFile, decompressedData, 0666)
	if err != nil {
		return err
	}
	fmt.Printf("Saved file: %s\n", decompressedFile)

	// Delete original file
	err = os.Remove(compressedFile)
	if err != nil {
		return err
	}

	// If new file's MagicID == OFS3, then extract everything from it.
	if path.Ext(decompressedFile) == ".ofs3" {
		fmt.Printf("Processing osf3: %s\n", decompressedFile)

		err := UnpackOFS3File(decompressedFile, path.Join(path.Dir(decompressedFile), "EXTRACTED_"+path.Base(decompressedFile)))
		if err != nil {
			return err
		}
	}

	return nil
}

// Comeon golang.. why do i need to define this
func readString(f io.Reader, order binary.ByteOrder, str *string) error {

	for {
		var c byte
		err := binary.Read(f, order, &c)
		if err != nil {
			return err
		}

		if c == 0x0 {
			break
		}

		*str += string(c)
	}

	return nil
}