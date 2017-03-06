package main

import (
	"encoding/json"
	"log"
	"net/http"
	// "strconv"

	"github.com/gorilla/websocket"
	"gopkg.in/mgo.v2/bson"
)

// const SHORT_URL = "http://mp.weixin.qq.com/s/WEBkpBjBdOAIXxu9fknV9w"
// const ITEM = `{"super_vote_item":[{"vote_id":684888407,"item_idx_list":{"item_idx":["0"]}}],"super_vote_id":684888406}`
// const DST_VOTES = 1
// const VOTE_URL = "https://mp.weixin.qq.com/s?__biz=MzA5NjYwOTg0Nw==&mid=2650886522&idx=1&sn=317f363e12cd7c45e6bbc0de9916a6c6&key=f6fc65d37e8c2007e879f47762586e65a02d8fbd5b84db235e00e511b8101f887e892a2554674628ca531decec74f300247b10a9d1bddcb0db5ed37662159345e43c794bdb7046a6a6c53cd203b232d1&ascene=1&uin=MTMwMzUxMjg3Mw%3D%3D&devicetype=Windows+7&version=61000603&pass_ticket=EnayxJ3mRIUH%2BQl8MDq4Bjq1qQJiB0M4Od8lSTPh3ejMZ1VSt03lQLCWB0LI5dKT"

// var gWsConns = map[string]*websocket.Conn{}

func main() {
	// mongo
	err := InitMongo("127.0.0.1")
	if err != nil {
		log.Fatalf("init mongo error: %v", err)
	}

	// beego.BConfig.WebConfig.Session.SessionOn = true
	// beego.BConfig.WebConfig.Session.SessionProvider 默认是 memory，目前支持还有 file、mysql、redis 等
	// beego.BConfig.WebConfig.Session.SessionGCMaxLifetime 默认3600秒
	http.HandleFunc("/api/login", Login)
	http.HandleFunc("/api/tasks", TasksHandle)
	http.HandleFunc("/api/parseurl", ParseUrl)
	http.HandleFunc("/api/submititem", SubmitItem)
	http.HandleFunc("/api/submittask", SubmitTask)

	http.HandleFunc("/api/users/changepassword", ChangePasswordHandle)

	http.HandleFunc("/api/users", UsersHandle)
	http.HandleFunc("/api/newuser", NewUser)

	// TODO websocket1: /api/ws/runner PC端连接，下发任务
	http.HandleFunc("/api/ws/runner", WsRunner)
	http.HandleFunc("/api/vote", RunnerVote)

	// TODO websocket2: /api/ws/web web端连接，实时查询状态
	http.HandleFunc("/api/ws/web", WsWeb)
	http.Handle("/", http.FileServer(http.Dir("../web")))

	const addr = ":8080"
	log.Printf("listen at %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func Login(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/login:")

	user := UserLogin(w, r)
	if user == nil {
		// 如果返回nil，说明失败，内部会回复响应，所以这里直接return
		return
	}

	w.Write([]byte(`{"ret":0,"msg":"login success"}`))
}

func TasksHandle(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/tasks:")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	// 获取任务时需要按照user获取
	voteInfos, err := QueryTasksByUser(user.UserName)
	if err != nil {
		log.Printf("QueryTasksByUser error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tasks := []map[string]interface{}{}
	for _, info := range voteInfos {
		task := map[string]interface{}{}
		task["title"] = info.Info["title"]
		task["votes"] = info.Votes
		task["curvotes"] = info.CurVotes
		tasks = append(tasks, task)
	}
	log.Printf("tasks: %+v", tasks)

	tasksBytes, err := json.Marshal(tasks)
	if err != nil {
		log.Printf("json.Marshal error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(tasksBytes)
}

// 根据url解析出投票信息
func ParseUrl(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/parseurl:")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	voteUrl := r.FormValue("url")
	if voteUrl == "" {
		log.Printf("url is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("voteUrl: %v", voteUrl)

	// 根据短url来获取投票信息
	task, err := NewTask(voteUrl)
	if err != nil {
		log.Printf("NewTask error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// log.Printf("task: %+v", task)

	// infoBytes, err := json.Marshal(task.Info)
	info := map[string]interface{}{}
	info["key"] = task.Key
	info["info"] = task.Info
	infoBytes, err := json.Marshal(info)
	if err != nil {
		log.Printf("marshal info error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(infoBytes)
}

// 提交投票对象
func SubmitItem(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/submititem:")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	w.Write([]byte("{}"))
}

// 提交任务
func SubmitTask(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/submittask:")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	// url
	voteUrl := r.FormValue("url")
	if voteUrl == "" {
		log.Printf("url is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("voteUrl: %v", voteUrl)

	// info
	infoStr := r.FormValue("info")
	if infoStr == "" {
		log.Printf("info is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info := map[string]interface{}{}
	// err := json.Unmarshal([]byte(infoStr), &info)
	err := jsonUnmarshal([]byte(infoStr), &info)
	if err != nil {
		log.Printf("json.Unmarshal info error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("info: %v", string(infoStr[:60]))

	// 取出key
	// key := info["key"].(string)
	key := r.FormValue("key")
	log.Printf("key: %v", key)
	delete(info, "key")

	// item
	itemStr := r.FormValue("item")
	if itemStr == "" {
		log.Printf("item is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// item := map[string]interface{}{}
	// // err = json.Unmarshal([]byte(itemStr), &item)
	// err = jsonUnmarshal([]byte(itemStr), &item)
	// if err != nil {
	// 	log.Printf("json.Unmarshal item error: %v", err)
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	return
	// }
	// log.Printf("item: %v", item)
	log.Printf("itemStr: %v", itemStr)

	// task
	taskStr := r.FormValue("task")
	if taskStr == "" {
		log.Printf("task is empty")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	task := map[string]interface{}{}
	// err = json.Unmarshal([]byte(taskStr), &task)
	err = jsonUnmarshal([]byte(taskStr), &task)
	if err != nil {
		log.Printf("json.Unmarshal task error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("task: %v", task)

	votes, _ := task["votes"].(json.Number).Int64()
	speed, _ := task["votespermin"].(json.Number).Int64()
	price, _ := task["price"].(json.Number).Float64()

	taskStruct := &Task{
		Id:  bson.NewObjectId(),
		Url: voteUrl,
		// Key:    GetKeyFromUrl(voteUrl),
		Key:  key,
		Info: info,
		// Item:   item,
		Item:   itemStr,
		User:   user.UserName,
		Votes:  uint64(votes),
		Price:  price,
		Speed:  uint64(speed),
		Status: "doing",
	}

	// 写到数据库中
	err = taskStruct.Insert()
	if err != nil {
		log.Printf("taskStruct.insert error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte("{}"))

	// 处理任务
	runnerCount := len(gRunners)
	if runnerCount == 0 {
		log.Printf("ERROR runner not found")
		return
	}

	runner := GetFreeRunner(key)
	if runner == nil {
		log.Printf("ERROR GetFreeRunner returns nil runner")
		return
	}

	runner.DispatchTask(taskStruct)

	log.Printf("dispatch task(%v,%v) to executer(%v)", taskStruct.Url, taskStruct.Votes, runner.Conn.RemoteAddr().String())
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func WsRunner(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/ws/runer:")

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	addr := ws.RemoteAddr().String()
	log.Printf("remoteaddr: %v", addr)
	// gWsConns[addr] = ws
	// defer delete(gWsConns, addr)

	for {
		msgtype, msgBytes, err := ws.ReadMessage()
		if err != nil {
			log.Printf("ws.ReadMessage error: %v", err)
			return
		}
		log.Printf("type: %v, content: %v", msgtype, string(msgBytes))

		msg := map[string]interface{}{}
		err = json.Unmarshal(msgBytes, &msg)
		if err != nil {
			log.Printf("json.Unmarshal error: %v", err)
			continue
		}

		if msg["cmd"].(string) == "login" {
			runner := &Runner{
				Conn: ws,
			}
			err = json.Unmarshal(msgBytes, runner)
			if err != nil {
				log.Printf("json.Unmarshal: %v", err)
				continue
			}
			log.Printf("runner: %+v", runner)

			gRunners[runner.Name] = runner
			defer delete(gRunners, runner.Name)
		} else if msg["cmd"].(string) == "vote_finish" {
			log.Printf("runner vote finish: %v,%+v", addr, msg)
			// 需要根据完成情况做调整
			TaskDispatch(msg["url"].(string))
		}

	}
}

// browser把url发过来
func RunnerVote(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/vote:")

	voteUrl := r.FormValue("url")
	if voteUrl == "" {
		log.Printf("url is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("voteUrl: %v", voteUrl)

	key := GetKeyFromUrl(voteUrl)
	if key == "" {
		log.Printf("get empty key from url %v", voteUrl)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 先回响应
	w.WriteHeader(http.StatusOK)

	task, err := QueryTaskByKey(key)
	if err != nil {
		log.Printf("QueryTaskByKey error: %v,%v", key, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	voter, err := task.NewVoter(voteUrl)
	if err != nil {
		log.Printf("newvoter error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// log.Printf("type of item: %T", task.Item["super_vote_id"])

	err = voter.Vote()
	if err != nil {
		log.Printf("vote error: %v", err)
		// 如果投票失败，则票数-1
		task.DecrVotes()
	}

	return
}

func WsWeb(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/ws/web")
	// TODO
}

func ChangePasswordHandle(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/users/changepassword:")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	oldpass := r.FormValue("old")
	if oldpass == "" {
		log.Printf("oldpassword is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if oldpass != user.Password {
		log.Printf("老密码不匹配")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	newpass := r.FormValue("new")
	if newpass == "" {
		log.Printf("新密码不能为空")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO 没使用旧
	err := user.ChangePassword(newpass)
	if err != nil {
		log.Printf("修改密码失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write([]byte("{}"))
}

func UsersHandle(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/users")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	users, err := user.QueryAllUsers()
	if err != nil {
		log.Printf("QueryAllUsers error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	by, err := json.Marshal(users)
	if err != nil {
		log.Printf("marshal users(%v) error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(by)
}

func NewUser(w http.ResponseWriter, r *http.Request) {
	log.Printf("/api/newuser:")

	user := UserLogin(w, r)
	if user == nil {
		return
	}

	if !user.IsAdmin {
		log.Printf("current user is not admin")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	if username == "" {
		log.Printf("username is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		log.Printf("password is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	newuser := &User{
		UserName: username,
		Password: password,
		IsAdmin:  false,
	}

	err := newuser.Insert()
	if err != nil {
		log.Printf("user insert error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write([]byte("{}"))
}
