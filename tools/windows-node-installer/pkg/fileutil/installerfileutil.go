package fileutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

func ReadInstanceInfo(info interface{}, path *string) (interface{}, error) {
	if _, err := os.Stat(*path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no InstanceInfo found at path '%v", *path)
	}
	content, err := ioutil.ReadFile(*path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at path '%v', %v", *path, err)
	}
	err = json.Unmarshal(content, &info)
	if err != nil {
		return nil, fmt.Errorf("failed to read json file at path '%v'", err)
	}
	return info, nil
}
