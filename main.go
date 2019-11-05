package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var num = regexp.MustCompile(`/`)
const ShellToUse = "sh"

type line struct {
	Ipaddr string	`json:"ipaddr"`
	Comment string	`json:"comment"`
}

type lines struct{
	Name string `json:"name"`
	Lines []line `json:"lines"`
}

var linesfile string = "/root/www/line.json"
var redirectconf string = "/root/config.ini"
var staticPath string = `/root/www/web`

func main()  {
	//创建路由表
	mux := http.NewServeMux()
	//注册路由
	mux.HandleFunc("/",router)

	//静态文件处理  涉及路径 路由 文件服务器
	h := http.FileServer(http.Dir(staticPath))

	mux.Handle("/web/",
		http.StripPrefix("/web/",h))
	//服务监听
	err1 := http.ListenAndServe(":8080",mux)
	if err1 != nil {
		log.Fatal(err1)
	}
}

func router(w http.ResponseWriter, r *http.Request)  {
	url := r.URL
	if strings.HasPrefix(fmt.Sprint(url), "/api") {
		api(w,r)
	}else {
		web(w,r)
	}
}

func api(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(fmt.Sprint(r.URL), "/api/getlines"):
		GET_getlines(w,r)
	case strings.HasPrefix(fmt.Sprint(r.URL), "/api/modlines"):
		POST_modline(w,r)
	case strings.HasPrefix(fmt.Sprint(r.URL), "/api/reboot"):
		POST_reboot(w,r)
	case strings.HasPrefix(fmt.Sprint(r.URL), "/api/getuseline"):
		GET_useline(w,r)
	case strings.HasPrefix(fmt.Sprint(r.URL), "/api/applayline"):
		POST_Applay_Lines(w,r)
	case strings.HasPrefix(fmt.Sprint(r.URL), "/api/stopspeed"):
		POST_Stop_speed(w,r)
	default:
		w.Write([]byte("request error"))
	}
}

func web(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	t, err := template.ParseFiles("web/index.html")
	if err != nil {
		log.Println("err")
	}
	t.Execute(w, nil)
}

func GET_getlines(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	//读取配置文件
	l := ReadLineFromFile()
	w.Write([]byte(l))
}

func POST_modline(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	s, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
	}

	linefile, err := os.OpenFile(linesfile, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	var str bytes.Buffer
	_ = json.Indent(&str, s, "", "	")
	fmt.Println(str.String())
	l := str.String()
	linefile.Write([]byte(l))
}

func POST_reboot(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	command := r.PostFormValue("command")
	if command == "reboot" {
		w.Write([]byte("systemctl will be reboot."))
		er, sout, eout := Shellout("/sbin/reboot")
		if er != nil {
			log.Println(er)
		}else if sout != "" {
			log.Println(sout)
		}else if eout != "" {
			log.Println(eout)
		}else {
			log.Println("rebooting!")
		}
	}else {
		w.Write([]byte("command error"))
	}
}

func GET_useline(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	f, err := os.Open(redirectconf)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var l = line{}
	rd := bufio.NewReader(f)
	for {
		ls, err := rd.ReadString('\n') //以'\n'为结束符读入一行

		if err != nil || io.EOF == err {
			break
		}
		if strings.HasPrefix(string(ls), "Servers=") {
			ipaddport := strings.Split(ls,"=")
			ip := strings.Replace(ipaddport[1], "\n", "", -1)
			l =  line{Ipaddr:ip}
		}
	}

	b, err := json.Marshal(l)
	if err != nil {
		fmt.Println(err)
	}
	w.Write(b)
}

func ReadLineFromFile() string {
	fi, err := os.Open(linesfile)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
	defer fi.Close()
	a,err := ioutil.ReadAll(fi)
	if err != nil {
		fmt.Println(err)
	}
	return string(a)
}

func Shellout(command string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(ShellToUse, "-c", command)
	cmd.Dir = "/root/ch_mod/"
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}

func POST_Applay_Lines(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	command := r.PostFormValue("command")
	if command == "applayline" {
		l := r.PostFormValue("line")
		mod_redirect_config(l)
		Shellout("/etc/init.d/redirect restart")
		Shellout("./mode.sh stop")
		Shellout("./mode.sh full")
		w.Write([]byte("applayline finish."))

	}else {
		w.Write([]byte("command error"))
	}
}

func mod_redirect_config(newl string) {
	f, err := os.Open(redirectconf)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var configstr string
	rd := bufio.NewReader(f)
	for {
		l, err := rd.ReadString('\n') //以'\n'为结束符读入一行

		if err != nil || io.EOF == err {
			break
		}

		if strings.HasPrefix(l, "Servers="){
			al := "Servers=" + newl +"\n"
			configstr = configstr + al
			continue
		}
		configstr = configstr + string(l)
	}
	linefile, err := os.OpenFile(redirectconf, os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	linefile.Write([]byte(configstr))
}

func POST_Stop_speed(w http.ResponseWriter, r *http.Request)  {
	log.Println("request domain ",r.Host,"URL: ",r.URL)
	command := r.PostFormValue("command")
	if command == "stopspeed" {
		Shellout("/etc/init.d/redirect restart")
		Shellout("./mode.sh stop")
		w.Write([]byte("stop speed."))
	}else {
		w.Write([]byte("command error"))
	}
}
