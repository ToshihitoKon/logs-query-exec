package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
)

func checkEmpty(v any, varName string) error {
	err := fmt.Errorf("%s is required", varName)
	val := reflect.ValueOf(v)

	switch val.Kind() {
	case reflect.Pointer:
		if v == nil || val.IsNil() {
			return err
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if val.Len() < 1 {
			return err
		}
	}

	return nil
}

func loadLambdaPayloadSample(filePath string) (*RequestEvent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	req := &RequestEvent{}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, err
	}

	return req, nil
}