package darktrader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

const FILE_PATH string = "/root/darktrader_tokens.json"

func WriteTokensToFile(pairs map[string]*DTPair) {
	f, err := os.Create(FILE_PATH)
	if err != nil {
		fmt.Println("error in writing tokens", err)
		return
	}

	var tokens []string
	for _, pair := range pairs {
		tokens = append(tokens, pair.token.Hex())
	}

	bytes, err := json.Marshal(tokens)

	_, err = f.Write(bytes)

	if err != nil {
		fmt.Println("error in writing tokens", err)
		return
	}

	if err := f.Close(); err != nil {
		fmt.Println("error in writing tokens", err)
		return
	}
}

func ReadTokensFromFile() []string {
	var tokens []string
	jsonFile, err := os.Open(FILE_PATH)

	if err != nil {
		fmt.Println("File not found for predefined tokens", err)
		return nil
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &tokens)
	if err != nil {
		fmt.Println("Parsing token json file error", err)
		return nil
	}
	return tokens
}
