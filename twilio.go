package main

import (
	"encoding/xml"
	"github.com/hiteshjoshi/gin"
	"net/http"
	"net/url"
	"fmt"
	"strings"
	"io/ioutil"
	"encoding/json"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
	"os"
)


type Record struct{
	XMLName xml.Name `xml:"Record"`
	Action  string `xml:"action,attr,omitempty"`
	Method  string `xml:"method,attr,omitempty"`
	MaxLength  string `xml:"maxLength,attr,omitempty"`
	FinishOnKey  string `xml:"finishOnKey,attr,omitempty"`
}

type TwiML struct {
  XMLName xml.Name `xml:"Response"`

  Say    string `xml:",omitempty"`
  Play   string `xml:"Play,omitempty"`
  Record   Record `xml:",omitempty"`
  Hangup  string `xml:"Hangup"`
}


type (
	Call struct {
		Id     bson.ObjectId `json:"id" bson:"_id"`
		Number   string        `json:"number" bson:"number"`
		Time time.Time        `json:"time" bson:"time"`
		Duration    string           `json:"duration" bson:"duration"`
		Pending    bool           `json:"pending" bson:"pending"`
		CallSid   string        `json:"call_sid" bson:"call_sid"`
		RecordingUrl  string `json:"recording_url" bson:"recording_url"`
		Reminder  string `json:"reminder" bson:"reminder"`
	}

	//CallController represents the controller for operating on the User resource
	CallController struct {
		session *mgo.Session
	}
)


func main() {

	fmt.Println(os.Getenv("PATH"))
	//start mongodb session
	session := getSession()
	defer session.Close()
	// Index
	index := mgo.Index{
		Key:        []string{"call_sid","id","reminder"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}

	err := session.DB("c100").C("recordings").EnsureIndex(index)
	if err != nil {
		panic(err)
	}

	//HTTP methods to handle cron events on HTTP
	router := gin.Default()

	router.GET("/",func(c *gin.Context){
		c.JSON(200, gin.H{
            "all": "ok",
        })
	})


	//SEND TWILM , XML to record the call.
	router.POST("/record", func(c *gin.Context) {
		twiml := TwiML{Say:"Please leave a message at the beep. Press the pound key when finished.",Record:Record{Action:"http://record.livetest.io/handle_recording",Method:"GET",MaxLength:"20",FinishOnKey:"#"}}
        c.XML(200, twiml)
    })


    router.POST("/play_recording/:reminder", func (c *gin.Context) {
    	reminder:=c.Param("reminder")
    	controller:=NewController(session)

    	call,err := controller.FindByReminderId(reminder)
    	if err!=nil{
			panic(err)
		}

    	twiml := TwiML{Say:"Your Care To Call reminder is here.",Play:call.RecordingUrl+".mp3"}
        c.XML(200, twiml)
    })


    //handle_recording, mp3 file etc
    router.GET("/handle_recording", func (c *gin.Context){
    	
		controller:=NewController(session)

		doc,err:=controller.Find(c.Query("CallSid"))

		if err!=nil{
			panic(err)
		}
		doc.RecordingUrl = c.Query("RecordingUrl")
		doc.Duration = c.Query("RecordingDuration")
		doc.Pending = false

		controller.Update(doc)

    	//for now lets give it back to the world
    	//fmt.Println(c.Query("CallSid"),c.Query("RecordingUrl"),c.Query("RecordingDuration"),c.Query("Digits"))
    	c.XML(200,gin.H{})
    })


    router.POST("/call_user", func (c *gin.Context) {
    	number:=c.PostForm("number")
    	reminder:=c.PostForm("reminder")

    	fmt.Println(reminder,number)
    	// Let's set some initial default variables
		accountSid := os.Getenv("ACCOUNT_SID")
		authToken := os.Getenv("AUTH_TOKEN")
		urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + accountSid + "/Calls.json"

		// Build out the data for our message
		v := url.Values{}
		v.Set("To",number)
		v.Set("From",os.Getenv("PHONE_NUMBER"))
		v.Set("Url","http://record.livetest.io/play_recording/"+reminder)
		rb := *strings.NewReader(v.Encode())

		// Create Client
		client := &http.Client{}

		req, _ := http.NewRequest("POST", urlStr, &rb)
		req.SetBasicAuth(accountSid, authToken)
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		// make request
		resp, _ := client.Do(req)
		if( resp.StatusCode >= 200 && resp.StatusCode < 300 ) {
			var data map[string]interface{}
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			err := json.Unmarshal(bodyBytes, &data)

			if( err == nil ) {
				//newcall.CallSid = data["sid"].(string)

				//controller.Save(newcall)

				fmt.Println(data["sid"])
			}
		} else {
			
			fmt.Println(resp.Status);
			c.JSON(400, gin.H{
	            "all": "ok",
	        })
		}


	})


    router.POST("/call", func (c *gin.Context) {

    	number:=c.PostForm("number")
    	reminder:=c.PostForm("reminder")


    	controller:=NewController(session)


    	newcall:= Call{Reminder:reminder,Id:bson.NewObjectId(),Number:number,Time:time.Now(),Pending:true}

    	// Let's set some initial default variables
		accountSid := os.Getenv("ACCOUNT_SID")
		authToken := os.Getenv("AUTH_TOKEN")
		urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + accountSid + "/Calls.json"

		// Build out the data for our message
		v := url.Values{}
		v.Set("To",number)
		v.Set("From",os.Getenv("PHONE_NUMBER"))
		v.Set("Url","http://record.livetest.io/record")
		rb := *strings.NewReader(v.Encode())

		// Create Client
		client := &http.Client{}

		req, _ := http.NewRequest("POST", urlStr, &rb)
		req.SetBasicAuth(accountSid, authToken)
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		// make request
		resp, _ := client.Do(req)
		if( resp.StatusCode >= 200 && resp.StatusCode < 300 ) {
			var data map[string]interface{}
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			err := json.Unmarshal(bodyBytes, &data)

			if( err == nil ) {
				newcall.CallSid = data["sid"].(string)

				controller.Save(newcall)

				fmt.Println(data["sid"])
			}
		} else {
			
			fmt.Println(resp.Status);
			c.JSON(400, gin.H{
	            "all": "ok",
	        })
		}

    })


    

	//start HTTP server at port 8000
	router.Run(":8000")
}





// NewController provides a reference to a CallController with provided mongo session
func NewController(s *mgo.Session) *CallController {
	return &CallController{s}
}


func (cc CallController) Save(call Call){

	err := cc.session.DB("c100").C("recordings").Insert(call)

	if err != nil {
		panic(err)
	}
}


func (cc CallController) FindByReminderId(reminder string) (Call,error){
	result := Call{}
	err := cc.session.DB("c100").C("recordings").Find(bson.M{"reminder": reminder}).One(&result)
	if err != nil {
		panic(err)
	}
	return result,err;
}

func (cc CallController) Find(CallSid string) (Call,error) {
	
	result := Call{}
	err := cc.session.DB("c100").C("recordings").Find(bson.M{"call_sid": CallSid}).One(&result)
	if err != nil {
		panic(err)
	}
	return result,err;
}


func (cc CallController) Update(Call Call) {
	
	//result := Call{}
	err := cc.session.DB("c100").C("recordings").Update(bson.M{"call_sid": Call.CallSid},Call)
	if err != nil {
		panic(err)
	}
}


// getSession creates a new mongo session and panics if connection error occurs
func getSession() *mgo.Session {
	fmt.Println(os.Getenv("MONGO_URL"),os.Getenv("TEST"))
	// Connect to our local mongo
	s, err := mgo.Dial(os.Getenv("MONGO_URL"))

	// Check if connection error, is mongo running?
	if err != nil {
		panic(err)
	}
	DB:=s.DB("c100")

	err = DB.Login(os.Getenv("MONGO_USER"), os.Getenv("MONGO_PASS"))
	if err != nil {
		panic(err)
	}

	s.SetMode(mgo.Monotonic, true)

	// Deliver session
	return s
}