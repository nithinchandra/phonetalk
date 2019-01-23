package phonetalk

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func init(){
	http.HandleFunc("/",handler)

}

const ( welcomeMsg  = `<?xml version="1.0" encoding="UTF-8"?>
	<Response>
		<Say>Hello how are you?</Say>
		<Record timeout="5" />
	</Response>`

		echoMsg =`<?xml version="1.0" encoding="UTF-8"?>
	<Response>
		<Play>%s</Play>
	</Response>`

	comeInMsg =`<?xml version="1.0" encoding="UTF-8"?>
	<Response>
		<Say>Welcome Home!</Say>
	</Response>`

	goAwayMsg =`<?xml version="1.0" encoding="UTF-8"?>
	<Response>
		<Say>Go Away</Say>
	</Response>`
)

func handler(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "text/xml")

	if err := r.ParseForm(); err != nil{
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rec := r.FormValue("RecordingUrl")

	if rec == ""{
		fmt.Fprint(w,welcomeMsg)
		return
	}
	
	c := appengine.NewContext(r)
	text, err := transcribe(c,rec)

	if err != nil{
		http.Error(w,"We could not transcribe", http.StatusInternalServerError)
		log.Printf("could not transcribe: %v",err)
		return
	}

	if text =="hello"{
		fmt.Fprintf(w, comeInMsg)
	}else{
		fmt.Fprintf(w, goAwayMsg)
	}


	fmt.Fprintf(w, echoMsg,rec)
}

func transcribe(c context.Context, url string) (string, error){
	url = "https://api.twilio.com/2010-04-01/Accounts/ACab457137961701c2918bc3e0222b5ab2/Recordings/RE493726fec50326d2b1b60a8163231619"
	b, err := fetchAudio(c, url)
	if err != nil{
		return  "", err
	}

	return fetchTranscription(c, b)
}

func fetchAudio(c context.Context, url string) ([]byte, error){
	client := urlfetch.Client(c)
	res, err := client.Get(url)
	if err != nil{
		return nil, fmt.Errorf("coudlnt fetch %v: %v",url,err)
	}
	if res.StatusCode != http.StatusOK{
		return  nil, fmt.Errorf("Fetched with status of %s",res.Status)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil{
		return nil, fmt.Errorf("coulsnt read response %v", err)
	}
	return b, nil
}

var speechURL  = "https://speech.googleapis.com/v1/speech:recognize?key="+ os.Getenv("SPEECH_API_KEY")

type speechReq struct {
	Config struct{
		Encoding string `json:"encoding"`
		SampleRate int `json:"sampleRate"`
	}`json:"config"`
	Audio struct{
		Content string `json:"content"`
	} `json:"audio"`
}
func fetchTranscription(c context.Context, b []byte) (string, error){
	var req speechReq
	req.Config.Encoding="LINEAR16"
	req.Config.SampleRate=8000
	req.Audio.Content = base64.StdEncoding.EncodeToString(b)

	j, err := json.Marshal(req)
	if err !=nil{
		return "", fmt.Errorf("Could not encode request: %v",err)
	}

	res, err := urlfetch.Client(c).Post(speechURL,"application/json",bytes.NewReader(j))
	if err != nil{
		return "",fmt.Errorf("could not transcribe: %v", err)
	}

	var data struct{
		Error struct{
			Code int
			Message string
			Status  string
		}
		Results []struct{
			Alternatives []struct{
				Transcript string
				Confidence float64
			}
		}
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil{
		return "",fmt.Errorf("could not decode speech resp: %v", err)
	}

	if data.Error.Code != 0{
		return "",fmt.Errorf("SpeechApi error: %d %s %s",data.Error.Code,data.Error.Status,data.Error.Message)

	}

	if len(data.Results) == 0 || len(data.Results[0].Alternatives) == 0{
		return "", fmt.Errorf("no transcriptions found")
	}

	return data.Results[0].Alternatives[0].Transcript, nil

	}
