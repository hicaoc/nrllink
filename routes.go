package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	jsoniter "github.com/json-iterator/go"
)

// type routes struct {
// 	Path      string `json:"path"`
// 	Component string `json:"component"`
// 	Hidden    bool `json:"hidden"`
// 	Children  []routes `json:"children"`
// }

type routes struct {
	Routes jsoniter.RawMessage `json:"routes" db:"routes"`
}

func getRoutes() *routes {
	r := &routes{}

	query := fmt.Sprintf("SELECT * from routes ")

	err := db.Get(r, query)
	if err != nil {
		log.Println("query routes err:", err, r)
	}
	return r

}

func setRoutes(route string) {

	r := &routes{}

	query := fmt.Sprintf("update  routes  set routes=%v", route)

	err := db.Get(r, query)
	if err != nil {
		log.Println("save routes err:", err, r)
	}

}

func (j *jsonapi) httpGetRoutes(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, ok := checktoken(w, req)

	if !ok {
		return
	}

	// type responseinfo struct {
	// 	Code int    `json:"code"`
	// 	Data routes `json:"data"`
	// }

	// r := responseinfo{Code: 20000, Data: getRoutes()}
	// rescode, _ := jsonextra.Marshal(r)
	// w.Write(rescode)
	res := fmt.Sprintf(`{"code":20000,"data":%v}`, string(getRoutes().Routes))
	w.Write([]byte(res))

}
func (j *jsonapi) httpSetRoutes(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if checkrole(u, []string{"admin"}) == false {
		w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"当前用户没有权限设置此参数\"}}"))
		return

	}

	result, _ := ioutil.ReadAll(req.Body)

	// req.Body.Close()

	// stb := &routes{}
	// err := jsonextra.Unmarshal(result, &stb)

	// if err != nil {
	// 	fmt.Println("opadd err :", err)
	// 	w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"新增用户失败\"}}"))
	// 	return
	// }

	// r := getRoleByID(stb.Position)
	// stb.Roles = []string{r.NameKey}

	// addUser(stb)
	if len(result) > 10 {
		setRoutes(string(result))
		addOperatorLog("", "routes修改成功", u)
		w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"routes修改成功\"}}"))
	} else {
		w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"routes修改失败\"}}"))
	}

}
