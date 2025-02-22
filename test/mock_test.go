package test

import (
	"github.com/agiledragon/gomonkey/v2"
	"io"
	"net/http"
	"testing"
)

func httpGetRequest(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func fetchResult(url string) (string, error) {
	rsp, err := httpGetRequest(url)
	if err != nil {
		return "", err
	}
	return string(rsp), nil
}

func TestAdd(t *testing.T) {
	mockData := []byte(`{"name":"lihua","age":32}`)
	mockData1 := []byte(`{"name":"hanmeimei","age":28}`)
	patch := gomonkey.ApplyFuncSeq(httpGetRequest, []gomonkey.OutputCell{
		{Values: gomonkey.Params{mockData, nil}},
		{Values: gomonkey.Params{mockData1, nil}},
	})
	defer patch.Reset()
	rsp, _ := fetchResult("http://www.baidu.com")
	t.Log(rsp)
	rsp, _ = fetchResult("http://www.baidu.com")
	t.Log(rsp)
}
