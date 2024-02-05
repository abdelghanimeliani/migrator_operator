package controllers
import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func DeletePod(namespace,podname string) (map[string]interface{}, error){
	fmt.Println("deleting original pod")
	baseURL := "http://172.17.63.163:8080/v1/pods"
	urlParams := url.Values{}
	urlParams.Set("namespace", namespace)
	urlParams.Set("name", podname)

	fullURL := fmt.Sprintf("%s?%s", baseURL, urlParams.Encode())

	req, err := http.NewRequest("DELETE", fullURL, nil)
	if err != nil {
		fmt.Println("panic 1")
		panic(err)
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("panic 2")
		panic(err)
		return nil, err
	}
	defer resp.Body.Close()

	var parsedResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&parsedResponse)
	if err != nil {
		fmt.Println("panic 3")
		panic(err)
		return nil, err
	}
	fmt.Println("pod deleted")
	return parsedResponse, nil
}