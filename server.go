package main

import (
	"encoding/xml"
	"expvar"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/subosito/gotenv"
)

var elapsed = expvar.NewFloat("elapsed") // nanoseconds
var failed = expvar.NewInt("failed")
var succeed = expvar.NewInt("succeed")

var payload = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
<soap:Body>
	  <ns6:Request >
			<Header>
				  <Version>1.0</Version>
				  <CommandID>PreValidation</CommandID>
				  <LanguageCode>id</LanguageCode>
				   <OriginatorConversationID>1541565624</OriginatorConversationID>
				  <ConversationID>p110001811071140235678911840</ConversationID>
				  <Caller>
						  <CallerType>2</CallerType>
						  <ThirdPartyID>POS_Broker</ThirdPartyID>
						  <Password>gMMqGGrKxsE=</Password>
				  </Caller>
				  <KeyOwner>1</KeyOwner>
				  <Timestamp>20181107114023</Timestamp>
			</Header>
			<Body>
				
			</Body>
	  </ns6:Request>
</soap:Body>
</soap:Envelope>`

type CreateSoapEnvelope struct {
	CreateBody createBody `xml:"Body"`
}
type createBody struct {
	Request createRequest `xml:"Request"`
}
type createRequest struct {
	Header createHeader `xml:"Header"`
}
type createHeader struct {
	CommandID                string `xml:"CommandID"`
	OriginatorConversationID string `xml:"OriginatorConversationID"`
	ConversationID           string `xml:"ConversationID"`
	Timestamp                string `xml:"Timestamp"`
}

func init() {
	gotenv.Load()

}

//func ExpvarHandler(w http.ResponseWriter, r *http.Request)
func main() {
	logSetup()

	//handler := http.NewServeMux()
	http.HandleFunc("/", handleRequestAndRedirect)
	//http.HandleFunc("/ping", ping)

	rto, err1 := strconv.Atoi(os.Getenv("RTO"))
	wto, err2 := strconv.Atoi(os.Getenv("WTO"))

	if err1 != nil || err2 != nil {
		log.Fatalf("Could not start server check parameter: %s\n", err1.Error())
	}
	s := &http.Server{
		Addr: getListenAddress(),
		//Handler:      handler,
		ReadTimeout:  time.Duration(rto) * time.Second,
		WriteTimeout: time.Duration(wto) * time.Second,
		//MaxHeaderBytes: 1 << 20,
	}

	err3 := s.ListenAndServe()
	if err3 != nil {
		log.Fatalf("Could not start server: %s\n", err3.Error())
	}
	//http.HandleFunc("/", handler)
	//http.ListenAndServe(":8080", nil)
}

// Get the port to listen on
func getListenAddress() string {
	port := os.Getenv("PORT")
	return ":" + port
}

// Given a request send it to the appropriate url
func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	//elapsed
	defer timeTrack(time.Now(), "handler")
	//success
	defer func() { succeed.Add(1) }()

	requestPayload := parseRequestBody(req)
	url := getProxyUrl(requestPayload.CommandID)
	logRequestPayload(requestPayload, url)
	//serveReverseProxy(url, res, req)

}

// Get the url for a given proxy condition
func getProxyUrl(proxyConditionRaw string) string {
	proxyCondition := strings.ToUpper(proxyConditionRaw)
	condition_url := os.Getenv(proxyCondition)
	default_condtion_url := os.Getenv("DEFAULT_URL")

	if condition_url != "" {
		return condition_url
	}
	return default_condtion_url
}

// Parse the requests body
func parseRequestBody(request *http.Request) createHeader {

	body, err1 := ioutil.ReadAll(request.Body)
	log.Printf("body %s", body)

	if err1 != nil {
		log.Printf("Error reading body: %v", err1)
		//panic(err1)
		defer func() { failed.Add(1) }()

	}
	var createEnv CreateSoapEnvelope
	err := xml.Unmarshal([]byte(body), &createEnv)
	if err != nil {
		log.Printf(err.Error())
		//panic(err)
		defer func() { failed.Add(1) }()
	}

	//log.Printf("%v\n", createEnv.CreateBody.Request.Header.CommandID)
	return createEnv.CreateBody.Request.Header
}

// Log the typeform payload and redirect url
func logRequestPayload(requestionPayload createHeader, proxyUrl string) {
	log.Printf("proxy_condition: %s, proxy_url: %s\n", requestionPayload.CommandID, proxyUrl)
}

func ping(w http.ResponseWriter, r *http.Request) {
	//elapsed
	defer timeTrack(time.Now(), "handler")
	w.Write([]byte("Alhamdulillah"))
	//success
	defer func() { succeed.Add(1) }()
	//failed
	defer func() { failed.Add(1) }()
}
func timeTrack(start time.Time, name string) {
	x := time.Since(start)
	elapsed.Add(float64(x))
	log.Printf("%s took %s", name, elapsed)

}

// Log the env variables required for a reverse proxy
func logSetup() {
	default_condtion_url := os.Getenv("DEFAULT_URL")
	log.Printf("Redirecting to Default url: %s\n", default_condtion_url)
}

/*
	Reverse Proxy Logic
*/

// Serve a reverse proxy for a given url
func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {

	// parse the url
	url, _ := url.Parse(target)
	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)

	// Update the headers to allow for SSL redirection
	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = url.Host
	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}
