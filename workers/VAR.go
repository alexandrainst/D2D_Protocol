package workers

import (
	"bytes"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"

	"github.com/alexandrainst/agentlogic"
)

func CheckGoal(goal agentlogic.Goal, position agentlogic.Vector, poi interface{}) bool {
	if goal.Do == "matchImage" {
		//poi is a base64 string
		imgString := poi.(string)

		//reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(imgString))
		// m, _, err := image.Decode(reader)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		request, err := newfileUploadRequest("http://localhost:8888/upload", "data", imgString)
		if err != nil {
			log.Fatal(err)
		}
		client := &http.Client{}
		resp, err := client.Do(request)
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Println(resp.StatusCode)
			if resp.StatusCode == 200 {
				return true
			}
		}

	}
	return false
}

// Creates a new file upload http request with optional extra params
func newfileUploadRequest(uri string, paramName string, imgString string) (*http.Request, error) {
	// file, err := os.Open("./workers/test.png")
	// if err != nil {
	// 	return nil, err
	// }
	// fileContents, err := ioutil.ReadAll(file)
	// if err != nil {
	// 	return nil, err
	// }
	// fi, err := file.Stat()
	// if err != nil {
	// 	return nil, err
	// }
	// file.Close()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, "dude")
	if err != nil {
		return nil, err
	}
	//part.Write(fileContents)
	part.Write([]byte(imgString))

	// for key, val := range params {
	// 	_ = writer.WriteField(key, val)
	// }
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	//return http.NewRequest("POST", uri, body)
	request, err := http.NewRequest("POST", uri, body)

	request.Header.Add("Content-Type", writer.FormDataContentType())
	return request, err
}
