package main

import (
	"os"
	"log"
	"net"
	"fmt"
	"time"
	"bytes"
	"bufio"
	"regexp"
	"encoding/json"
)

const (
	_LT		= "\r\n"            	// packet line separator
	_LS		= "\x0D\x0A"	    	// Line serarators
	_KVT 		= ":"              	// header value separator
	_READ_BUF     	= 512              	// buffer size for socket reader
	_CMD_END      	= "--END COMMAND--"	// Asterisk command data end
	_ACSC		= "CoreShowChannels"	// Const Action
	_AQS		= "QueueStatus"		// Const Action
)

var (
	TCM = make(map[string][]map[string]string) //TELNET CONNECT MAP
	LOGDIR = ""
	_PT_BYTES = []byte(_LT + _LT) // packet separator
	stdlog,
	errlog *log.Logger
	AMIhost, AMIuser, AMIpass, AMIport string
	CHREX1, CHREX2, CHREX3 string
)

type Config struct  {
	ZabbixAmi ZabbixAmi
	LogDir LogDir
	ZabbixCheck ZabbixCheck
}

type ZabbixAmi struct {
	RemotePort string
	RemoteHost string
	Username   string
	Password   string
}

type LogDir struct {
	Path string
}

type ZabbixCheck struct {
	ChanRex1 string
	ChanRex2 string
	ChanRex3 string
}

type Message map[string]string

func amiActionResponse(mm map[string]string, action string, arg string) {
	conn, _ := net.Dial("tcp", AMIhost+":"+AMIport)
	fmt.Fprintf(conn, "Action: Login"+_LT)
	fmt.Fprintf(conn, "Username: "+AMIuser+_LT)
	fmt.Fprintf(conn, "Secret: "+AMIpass+_LT+_LT)
	buf0 := bytes.NewBufferString("")
	for k, v := range mm {
		buf0.Write([]byte(k))
		buf0.Write([]byte(_KVT))
		buf0.Write([]byte(v))
		buf0.Write([]byte(_LT))
	}
	buf0.Write([]byte(_LT))
	fmt.Fprintf(conn, buf0.String())
	fmt.Fprintf(conn, "Action: Logoff"+_LT+_LT)
	r := bufio.NewReader(conn)
	pbuf := bytes.NewBufferString("")
	buf := make([]byte, _READ_BUF)
	outcnt := make([]string, 0)
	incnt := make([]string, 0)
	qcall := ""
	for {
		rc, err := r.Read(buf)
		if err != nil {
			break
		}
		wb, err := pbuf.Write(buf[:rc])
		if err != nil || wb != rc { // can't write to data buffer, just skip
			continue
		}
		for pos := bytes.Index(pbuf.Bytes(), _PT_BYTES); pos != -1; pos = bytes.Index(pbuf.Bytes(), _PT_BYTES) {
			bp := make([]byte, pos + len(_PT_BYTES))
			r, err := pbuf.Read(bp)// reading packet to separate puffer
			if err != nil || r != pos + len(_PT_BYTES) { // reading problems, just skip
				continue
			}
			m := make(Message)
			for _, line := range bytes.Split(bp, []byte(_LT)) {
				if len(line) == 0 {
					continue
				}
				kvl := bytes.Split(line, []byte(_KVT+" "))
				if len(kvl) == 1 {
					if string(line) != _CMD_END {
						m["CmdData"] += string(line)
					}
					continue
				}
				k := bytes.TrimSpace(kvl[0])
				v := bytes.TrimSpace(kvl[1])
				m[string(k)] = string(v)
			}
			if action == _ACSC {
				fmt.Println(m)
				if arg == "out" {
					if xx, yy := regexp.MatchString(`` + CHREX1 + `\S*|` + CHREX2 + `\S*`, m["Channel"]); xx {
						if xx == true {
							outcnt = append(outcnt, m["Channel"])
						}
						if yy != nil {

						}
					}
				} else if arg == "in" {
					if xx, yy := regexp.MatchString(`` + CHREX3 + `\S*`, m["Channel"]); xx {
						if xx == true {
							incnt = append(incnt, m["Channel"])
						}
						if yy != nil {

						}
					}
				}
			} else if action == _AQS {
				if _, ok := m["Calls"]; ok {
					qcall = m["Calls"]
				}
			}
			
		}
	}
	if action == _ACSC && arg == "out" {
		fmt.Println(len(outcnt))
	} else if action == _ACSC && arg == "in" {
		fmt.Println(len(incnt))
	} else if action == _AQS {
		fmt.Println(qcall)
	}
}

func init() {
	file, e1 := os.Open("/etc/asterisk/asterisk_config.json")
	if e1 != nil {
		fmt.Println("Error: ", e1)
	}
	decoder := json.NewDecoder(file)
	conf := Config{}
	err := decoder.Decode(&conf)
	if err != nil {
		fmt.Println("Error: ", err)
	}
	AMIhost = conf.ZabbixAmi.RemoteHost
	AMIport = conf.ZabbixAmi.RemotePort
	AMIuser = conf.ZabbixAmi.Username
	AMIpass = conf.ZabbixAmi.Password
	LOGDIR = conf.LogDir.Path
	CHREX1 = conf.ZabbixCheck.ChanRex1
	CHREX2 = conf.ZabbixCheck.ChanRex2
	CHREX3 = conf.ZabbixCheck.ChanRex3
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime)
}

func main() {
	if len(os.Args) == 2 {
		arg1 := os.Args[1]
		switch arg1 {
		case "channels_out" :
			CoreShowChannels("out")
		case "channels_in" :
			CoreShowChannels("in")
		default:

		}
	} else if len(os.Args) == 3 {
		arg1 := os.Args[1]
		arg2 := os.Args[2]
		switch arg1 {
		case "queue_status" :
			QueueStatus(arg2)
		}
	}
}

func CoreShowChannels(s string) {
	var csc = make(map[string]string)
	csc["Action"] = _ACSC
	amiActionResponse(csc, _ACSC, s)
}

func QueueStatus(q string) {
	var qs = make(map[string]string)
	qs["Action"] = _AQS
	qs["Queue"] = q
	s := ""
	amiActionResponse(qs, _AQS, s)
}

func LoggerMap(s map[string]string) {
	tf := timeFormat()
	f, _ := os.OpenFile(LOGDIR+os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	log.SetOutput(f)
	log.Print(tf)
	log.Print(s)
	fmt.Println(s)
}

func LoggerString(s string) {
	tf := timeFormat()
	f, _ := os.OpenFile(LOGDIR+os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	log.SetOutput(f)
	log.Print(tf)
	log.Print(s)
	fmt.Println(s)
}

func timeFormat() (string) {
	t := time.Now()
	tf := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return tf
}
