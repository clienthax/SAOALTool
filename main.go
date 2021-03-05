package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	app := &cli.App{
		Name: "DecompressCriLayla",
	}

	app.Commands = []*cli.Command{
		&cmdDecompress,
		&cmdExtractOFS3,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

var cmdDecompress = cli.Command{
	Name: "decompress",
	Usage: "Decompress file",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name: "in",
			Required: true,
		},
	},
	Action: decompressFile,
}

var cmdExtractOFS3 = cli.Command{
	Name: "extract-osf3",
	Usage: "Extracts OSF3",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name: "in",
			Required: true,
		},
	},
	Action: extractOSF3,
}

func extractOSF3(c *cli.Context) error {
	inFilePath := c.Path("in")
	outDir := inFilePath+".out"

	err := UnpackOFS3File(inFilePath, outDir)
	if err != nil {
		return err
	}

	return nil
}

func decompressFile(c *cli.Context) error {
	inFilePath := c.Path("in")
	inFileData, err := ioutil.ReadFile(inFilePath)
	if err != nil {
		return err
	}

	// Xor header 16 bytes with key \0xff
	encHeader := inFileData[0:16]
	decHeader := Xor(encHeader, "\xff")

	// Replace with fixed header
	for i := 0; i < len(decHeader); i++ {
		inFileData[i] = decHeader[i]
	}

	decompressedData, err := DecompressCRILAYLA(inFileData)
	if err != nil {
		return err
	}

	// TODO check for container types.. (OSF3) and spit out some text if detected


	// Write output
	outPath := inFilePath+".out"

	// Check out if the decompressed file is a ".ofs3" and extract it if so
	reader := bytes.NewReader(decompressedData)
	var magic [4]byte
	binary.Read(reader, binary.LittleEndian, &magic)
	magicStr := string(magic[:])
	if magicStr == "OFS3" {
		fmt.Printf("Processing ofs3\n")
		err := UnpackOFS3(decompressedData, outPath)
		if err != nil {
			return err
		}
	} else {
		err = ioutil.WriteFile(outPath, decompressedData, 0644)
		if err != nil {
			return err
		}

		fmt.Printf("Saved to %s\n", outPath)
	}

	return nil
}

func Xor(input []byte, key string) (output []byte) {
	output = make([]byte, len(input))
	for i := 0; i < len(input); i++ {
		output[i] = input[i] ^ key[i % len(key)]
	}

	return output
}