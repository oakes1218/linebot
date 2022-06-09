package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	_ "github.com/joho/godotenv/autoload"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	bot     *linebot.Client
	tgbot   *tgbotapi.BotAPI
	tbotErr error
	botErr  error
	loc     *time.Location
	client  = &http.Client{}
	srv     *sheets.Service
)

const (
	chatID        = 193618166
	tgToken       = "1394548836:AAHdBSpf4QnA5Rt7xsLInEFDLMZ6i41Z0fY"
	spreadsheetId = "1sXc7hN7V7TV7lgF4bw1qz6INMfEb2ThUbPT_O5IeYuo"
	readRange     = "行程表!A1:A2"
)

// 資料存放記憶體
var (
	sMg = make([]MemGroup, 0)
	sA  = make([]Activity, 0)
)

type MemGroup struct {
	Member string
	Date   string
	Clock  string
	Number string
}

type Activity struct {
	Number int64
	Name   string
	Clock  string
	Times  string
}

func SetWeekGroup(mem, dt, ck, id string) (mg MemGroup) {
	mg.Member = mem
	mg.Date = dt
	mg.Clock = ck
	mg.Number = id

	return mg
}

// bot.ReplyMessage抽成func
func reply(event *linebot.Event, sentMsg ...linebot.SendingMessage) {
	if _, err := bot.ReplyMessage(event.ReplyToken, sentMsg...).Do(); err != nil {
		log.Println(err.Error())
		sendMsg("reply error : " + err.Error())
	}
}

func schedule(dateTime string, event *linebot.Event) {
	tt, err := time.ParseInLocation("2006-01-02 15:04:05", dateTime+":00", loc)
	if err != nil {
		log.Println(err.Error())
		sendMsg("schedule error : " + err.Error())
	}
	// 設定提醒清除排程
	go func() {
		stopTime := (tt.Unix() - time.Now().In(loc).Unix())
		//過期十分鐘自動刪除
		time.Sleep((time.Duration(stopTime) + 60) * time.Second)
		for k, v := range sA {
			if tt.Unix() == v.Number {
				sA = append(sA[:k], sA[k+1:]...)
			}
		}

		for k, v := range sMg {
			if strconv.FormatInt(tt.Unix(), 10) == v.Number {
				sMg = append(sMg[:k], sMg[k+1:]...)
			}
		}
		sendMsg("刪除逾時活動")
		// LOOP:
		// 	for {
		// 		for k, v := range sA {
		// 			if tt.Unix()-60*60 == time.Now().Unix() {
		// 				reply(event, sentMsg...)
		// 			}

		// 			if tt.Unix() == v.Number {
		// 				sA = append(sA[:k], sA[k+1:]...)
		// 			}
		// 		}

		// 		for k, v := range sMg {
		// 			if strconv.FormatInt(tt.Unix(), 10) == v.Number {
		// 				sMg = append(sMg[:k], sMg[k+1:]...)
		// 				sendMsg("刪除逾時活動")
		// 				break LOOP
		// 			}
		// 		}
		// 	}
	}()
}

func sendMsg(msg string) {
	newMsg := tgbotapi.NewMessage(chatID, msg)
	_, err := tgbot.Send(newMsg)
	if err != nil {
		log.Println(err.Error())
		return
	}
}

func memList() string {
	var tital, msg, allmsg string
	for _, v := range sA {
		tital += "活動名稱 : " + v.Name + " 時間 : " + v.Clock + " " + v.Times + " \n"
		for _, v1 := range sMg {
			if v.Clock == v1.Date && v.Times == v1.Clock && strconv.FormatInt(v.Number, 10) == v1.Number {
				msg += "人員 : " + v1.Member + " \n"
			}
		}
		allmsg += tital
		allmsg += "======================\n"
		allmsg += msg
		allmsg += "\n"
		tital = ""
		msg = ""
	}

	return allmsg
}

func actList() *linebot.TemplateMessage {
	var cc []*linebot.CarouselColumn
	picture := "https://upload.cc/i1/2022/06/01/1ryUBP.jpeg"
	if len(sA) == 0 {
		return nil
	}

	for _, v := range sA {
		cc = append(cc, linebot.NewCarouselColumn(
			picture,
			v.Clock+" "+v.Times,
			v.Name,
			linebot.NewPostbackAction("參加", v.Clock+"&"+v.Times+"&參加&"+v.Name+"&"+strconv.FormatInt(v.Number, 10), "", "", "", ""),
			linebot.NewPostbackAction("取消", v.Clock+"&"+v.Times+"&取消&"+v.Name+"&"+strconv.FormatInt(v.Number, 10), "", "", "", ""),
			linebot.NewPostbackAction("刪除活動", strconv.FormatInt(v.Number, 10)+"&刪除&"+v.Clock+"&"+v.Times+"&"+v.Name, "", "", "", ""),
		))
	}

	template := linebot.NewCarouselTemplate(cc...)
	msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)

	return msg
}

func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getEx(srv *sheets.Service) (mem, act string) {
	// Prints the names and majors of students in a sample spreadsheet:
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		fmt.Println(resp.Values)
		// fmt.Println(resp.Values[1][0])
		// for _, row := range resp.Values {
		// 	// Print columns A and E, which correspond to indices 0 and 4.
		// 	fmt.Printf("%s, %s\n", row[0], row[4])
		// }
		return resp.Values[0][0].(string), resp.Values[1][0].(string)
	}

	return
}

func insertEx(srv *sheets.Service, mem, act string) {
	var wR string
	data1D := make([]string, 0)
	if mem != "" {
		data1D = append(data1D, mem)
		wR = "行程表!A1"
	}

	if act != "" {
		data1D = append(data1D, act)
		wR = "行程表!A2"
	}

	s1D := make([]interface{}, len(data1D))
	for i, v := range data1D {
		s1D[i] = v
	}

	s2D := [][]interface{}{}
	s2D = append(s2D, s1D)
	vr := sheets.ValueRange{
		MajorDimension: "COLUMNS",
		Values:         s2D,
	}

	_, err := srv.Spreadsheets.Values.Update(spreadsheetId, wR, &vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Println(err.Error())
	}
}

func NewSrv() *sheets.Service {
	ctx := context.Background()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	return srv
}

func logMemList() string {
	s, err := json.Marshal(sMg)
	if err != nil {
		log.Printf("Error: %s", err)
		sendMsg("json.Marshal err : " + err.Error())
		return ""
	}

	return string(s)
}

func logActList() string {
	s, err := json.Marshal(sA)
	if err != nil {
		log.Printf("Error: %s", err)
		sendMsg("json.Marshal err : " + err.Error())
		return ""
	}

	return string(s)
}

func main() {
	//server重啟發tg
	tgbot, tbotErr = tgbotapi.NewBotAPI(tgToken)
	if tbotErr != nil {
		log.Panic(tbotErr)
	}

	tgbot.Debug = true

	go func() {
		quit := make(chan os.Signal, 1)
		errs := make(chan error, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-quit)
		sendMsg("line bot重啟...")

		c := <-errs
		sendMsg("中斷訊號 : " + c.Error())
	}()

	//設定時區 timer定時喚醒heroku
	loc, _ = time.LoadLocation("Asia/Taipei")
	ticker := time.NewTicker(9 * 60 * time.Second)
	defer ticker.Stop()
	go runtime(ticker, client)
	//讀取Excel
	NewSrv()
	mem, act := getEx(srv)
	if mem != "" && act != "" {
		m := []MemGroup{}
		mErr := json.Unmarshal([]byte(mem), &m)
		a := []Activity{}
		aErr := json.Unmarshal([]byte(act), &a)

		if mErr != nil || aErr != nil {
			log.Println(mErr, aErr)
			sendMsg("Unmarshal err : " + mErr.Error() + aErr.Error())
			return
		}
		sMg = m
		sA = a
		log.Println(sMg)
		log.Println(sA)
	}

	bot, botErr = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))
	if botErr != nil {
		log.Println(botErr.Error())
		return
	}

	r := gin.Default()
	r.POST("/callback", callbackHandler)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}

// 防止heroku休眠
func runtime(ticker *time.Ticker, client *http.Client) {
	for range ticker.C {
		resp, err := client.Get("https://linesebot.herokuapp.com/ping")
		if err != nil {
			log.Println(err.Error())
		}

		if resp.StatusCode == 200 {
			log.Println("喚醒heroku")
		} else {
			sitemap, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
				return
			}
			log.Println(string(sitemap))
		}
	}
}

func callbackHandler(c *gin.Context) {
	defer c.Request.Body.Close()
	// 接收請求
	events, err := bot.ParseRequest(c.Request)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		return
	}

	for _, event := range events {
		switch event.Type {
		case linebot.EventTypePostback:
			//判斷群組或個人取使用者名稱
			var userName string
			if event.Source.GroupID == "" {
				res, err := bot.GetProfile(event.Source.UserID).Do()
				if err != nil {
					log.Println(err.Error())
				}
				userName = res.DisplayName
			} else {
				res, err := bot.GetGroupMemberProfile(event.Source.GroupID, event.Source.UserID).Do()
				if err != nil {
					log.Println(err.Error())
				}
				userName = res.DisplayName
			}

			if event.Postback.Data != "" {
				str := strings.Split(event.Postback.Data, "&")
				if str[1] == "刪除" {
					for k, v := range sA {
						if strconv.FormatInt(v.Number, 10) == str[0] {
							sA = append(sA[:k], sA[k+1:]...)
						}
					}

					for k, v := range sMg {
						if v.Number == str[0] {
							sMg = append(sMg[:k], sMg[k+1:]...)
						}
					}

					if actList() == nil {
						reply(event, linebot.NewTextMessage(userName+" "+str[1]+" 活動 : "+str[4]+" 時段 : "+str[2]+" "+str[3]))
					} else {
						reply(event, linebot.NewTextMessage(userName+" "+str[1]+" 活動 : "+str[4]+" 時段 : "+str[2]+" "+str[3]), actList())
						insertEx(srv, "", logActList())
					}

					return
				}

				for k, v := range sMg {
					if v.Member == userName && v.Date == str[0] && v.Clock == str[1] && v.Number == str[4] {
						if str[2] == "參加" {
							return
						} else if str[2] == "取消" {
							sMg = append(sMg[:k], sMg[k+1:]...)
							reply(event, linebot.NewTextMessage(userName+" "+str[2]+" 活動 : "+str[3]+" 時段 : "+str[0]+" "+str[1]), linebot.NewTextMessage(memList()))
							return
						}
					}
				}

				if str[2] == "取消" {
					return
				}

				wg := SetWeekGroup(userName, str[0], str[1], str[4])
				sMg = append(sMg, wg)
				reply(event, linebot.NewTextMessage(userName+" "+str[2]+" 活動 : "+str[3]+" 時段 : "+str[0]+" "+str[1]), linebot.NewTextMessage(memList()))
				insertEx(srv, logMemList(), "")
			}
		case linebot.EventTypeMessage:
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if message.Text == "功能列表" {
					leftBtn := linebot.NewMessageAction("查看活動", "查看活動")
					rightBtn := linebot.NewMessageAction("參加人員", "參加人員")
					template := linebot.NewConfirmTemplate("新增活動指令： \n格式 ： date&time&activity \nex. 2022-01-01&00:00&散步步", leftBtn, rightBtn)
					msg := linebot.NewTemplateMessage("Sorry :(, please update your app.", template)
					reply(event, msg)
				}

				if message.Text == "LoG" {
					sm := logMemList()
					sa := logActList()
					reply(event, linebot.NewTextMessage(sm), linebot.NewTextMessage(sa))
				}

				if message.Text == "clearAll" {
					sMg = sMg[:0]
					sA = sA[:0]
					reply(event, linebot.NewTextMessage("Success clearAll"))
				}

				if message.Text == "參加人員" {
					if len(sMg) == 0 {
						reply(event, linebot.NewTextMessage("無參加人員"))
					} else {
						reply(event, linebot.NewTextMessage(memList()))
					}
				}

				if message.Text == "查看活動" {
					if len(sA) == 0 {
						reply(event, linebot.NewTextMessage("無活動列表"))
						return
					}
					reply(event, actList())
				}

				if message.Text != "" {
					var msg string
					sa := strings.Split(message.Text, "&")
					if len(sa) == 3 {
						dRegex := `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`
						tRegex := `^[0-9]{2}:[0-9]{2}$`
						rd, rErr := regexp.Compile(dRegex)
						rt, tErr := regexp.Compile(tRegex)
						if rErr != nil || tErr != nil {
							log.Println(err.Error())
							return
						}

						if !rd.MatchString(sa[0]) {
							msg += "日期格式錯誤 "
						}

						if !rt.MatchString(sa[1]) {
							msg += "時間格式錯誤 "
						}

						if msg == "" {
							var ac Activity
							tt, err := time.ParseInLocation("2006-01-02 15:04:05", sa[0]+" "+sa[1]+":00", loc)
							if err != nil {
								sendMsg("ParseInLocation err : " + err.Error())
							}

							ac.Number = tt.Unix()
							ac.Name = sa[2]
							ac.Clock = sa[0]
							ac.Times = sa[1]
							sA = append(sA, ac)
							msg += "新增活動成功"
							schedule(sa[0]+" "+sa[1], event) //, linebot.NewTextMessage("溫馨提醒 : "+sa[2]+"活動一小時後開始"))
							reply(event, linebot.NewTextMessage(msg), actList())
							insertEx(srv, "", logActList())

							return
						}
						reply(event, linebot.NewTextMessage(msg))
					}
				}
			}
		default:
			log.Printf("Unknown event: %v", event)
			sendMsg("Unknown event")
		}
	}
}
