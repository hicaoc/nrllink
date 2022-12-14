package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

// //CompareFunc 获取index
// type CompareFunc func(interface{}, interface{}) int

// func indexOf(a []interface{}, e interface{}, cmp CompareFunc) int {
// 	n := len(a)
// 	var i int
// 	for ; i < n; i++ {
// 		if cmp(e, a[i]) == 0 {
// 			return i
// 		}
// 	}
// 	return -1
// }

func (j *jsonapi) httpOperatorLogList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	//list := make([]OperatorLog, 0)
	var total = 0

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("operater log  query err :", err)
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"操作日记查询参数错误"}}`))
		return
	}

	ww, p, _ := queryToWhere("", *stb)

	list, total := getOperatorLog(ww, p, u)

	// list := getOperatorLog()

	//fmt.Println(emplist)

	rescode, _ := jsonextra.Marshal(list)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

// func (j *jsonapi) httpsavesetup(w http.ResponseWriter, req *http.Request) {
// 	sethttphead(w)

// 	u, ok := checktoken(w, req)
// 	if !ok {
// 		return
// 	}

// 	if checkrole(u, "admin") == false {
// 		w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"当前用户没有权限设置此参数\"}}"))
// 		return

// 	}

// 	//req.ParseForm()

// 	//points := strings.TrimSpace(req.Form.Get("points"))
// 	//perater := strings.TrimSpace(req.Form.Get("uid"))

// 	//conf.points = points

// 	w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"设置成功\"}}"))

//}
