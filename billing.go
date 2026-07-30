package main

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	billingStatusNotPay  = "NOTPAY"
	billingStatusSuccess = "SUCCESS"
)

type billingPackage struct {
	ID             int    `json:"id" db:"id"`
	Name           string `json:"name" db:"name"`
	Months         int    `json:"months" db:"months"`
	UnitPriceCents int    `json:"unit_price_cents" db:"unit_price_cents"`
	PriceCents     int    `json:"price_cents" db:"price_cents"`
	Status         int    `json:"status" db:"status"`
	Note           string `json:"note" db:"note"`
	CreateTime     string `json:"create_time" db:"create_time"`
	UpdateTime     string `json:"update_time" db:"update_time"`
}

type billingOrder struct {
	ID            int    `json:"id" db:"id"`
	OutTradeNo    string `json:"out_trade_no" db:"out_trade_no"`
	UserID        int    `json:"user_id" db:"user_id"`
	CallSign      string `json:"callsign" db:"callsign"`
	PackageID     int    `json:"package_id" db:"package_id"`
	Months        int    `json:"months" db:"months"`
	AmountCents   int    `json:"amount_cents" db:"amount_cents"`
	Status        string `json:"status" db:"status"`
	PrepayID      string `json:"prepay_id" db:"prepay_id"`
	CodeURL       string `json:"code_url" db:"code_url"`
	TransactionID string `json:"transaction_id" db:"transaction_id"`
	PayerOpenID   string `json:"payer_openid" db:"payer_openid"`
	PaidAt        string `json:"paid_at" db:"paid_at"`
	ExpireBefore  string `json:"expire_before" db:"expire_before"`
	ExpireAfter   string `json:"expire_after" db:"expire_after"`
	RawNotify     string `json:"raw_notify" db:"raw_notify"`
	CreateTime    string `json:"create_time" db:"create_time"`
	UpdateTime    string `json:"update_time" db:"update_time"`
}

type billingOrderReq struct {
	PackageID  int    `json:"package_id"`
	OutTradeNo string `json:"out_trade_no"`
}

func billingEnabled() bool {
	return conf.Billing.Enabled
}

func billingExpireRecheckDuration() time.Duration {
	secs := conf.Billing.AccountExpireRecheckSecs
	if secs <= 0 {
		secs = 300
	}
	return time.Duration(secs) * time.Second
}

func parseAccountExpireTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func isUserExpired(u *userinfo, now time.Time) bool {
	if u == nil {
		return false
	}
	expireAt, ok := parseAccountExpireTime(u.ExpireTime)
	return ok && !expireAt.After(now)
}

func refreshDeviceAccountExpired(dev *deviceInfo, now time.Time) bool {
	if dev == nil || !billingEnabled() {
		if dev != nil {
			dev.AccountExpired = false
		}
		return false
	}

	if !dev.LastExpireCheck.IsZero() && now.Sub(dev.LastExpireCheck) < billingExpireRecheckDuration() {
		return dev.AccountExpired
	}

	dev.LastExpireCheck = now
	u, err := getuser(dev.CallSign)
	if err != nil {
		log.Printf("billing: query account expire failed for %s: %v", dev.CallSign, err)
		return dev.AccountExpired
	}

	expired := isUserExpired(u, now)
	dev.AccountExpired = expired
	if old, ok := userlist.Load(u.CallSign); ok {
		old.(*userinfo).ExpireTime = u.ExpireTime
	}
	return expired
}

func removeExpiredDeviceFromPool(dev *deviceInfo, gp *group) {
	if dev == nil {
		return
	}
	if dev.udpAddr != nil && gp != nil && gp.connPool != nil {
		gp.connPool.removeDevice(dev.udpAddr.String())
	}
	dev.ISOnline = false
	dev.udpAddr = nil
}

func queryBillingPackages(includeDisabled bool) ([]*billingPackage, error) {
	where := ""
	if !includeDisabled {
		where = "WHERE status=1"
	}
	rows, err := db.Query(`SELECT id,name,months,unit_price_cents,price_cents,status,note,create_time,update_time
		FROM billing_packages ` + where + ` ORDER BY months ASC,id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*billingPackage{}
	for rows.Next() {
		p := &billingPackage{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Months, &p.UnitPriceCents, &p.PriceCents, &p.Status, &p.Note, &p.CreateTime, &p.UpdateTime); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func getBillingPackage(id int) (*billingPackage, error) {
	p := &billingPackage{}
	row := db.QueryRow(`SELECT id,name,months,unit_price_cents,price_cents,status,note,create_time,update_time
		FROM billing_packages WHERE id=?`, id)
	if err := row.Scan(&p.ID, &p.Name, &p.Months, &p.UnitPriceCents, &p.PriceCents, &p.Status, &p.Note, &p.CreateTime, &p.UpdateTime); err != nil {
		return nil, err
	}
	return p, nil
}

func normalizeBillingPackage(p *billingPackage) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("套餐名称不能为空")
	}
	if p.Months <= 0 {
		return errors.New("套餐月份必须大于0")
	}
	if p.UnitPriceCents <= 0 {
		p.UnitPriceCents = conf.Billing.PackageUnitPriceCents
	}
	if p.UnitPriceCents <= 0 {
		return errors.New("套餐月单价必须大于0")
	}
	p.PriceCents = p.Months * p.UnitPriceCents
	if p.Status != 0 {
		p.Status = 1
	}
	return nil
}

func createBillingPackage(p *billingPackage) error {
	if err := normalizeBillingPackage(p); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO billing_packages(name,months,unit_price_cents,price_cents,status,note,create_time,update_time)
		VALUES(?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		p.Name, p.Months, p.UnitPriceCents, p.PriceCents, p.Status, p.Note)
	return err
}

func updateBillingPackage(p *billingPackage) error {
	if p.ID <= 0 {
		return errors.New("套餐ID不能为空")
	}
	if err := normalizeBillingPackage(p); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE billing_packages SET name=?,months=?,unit_price_cents=?,price_cents=?,status=?,note=?,update_time=CURRENT_TIMESTAMP WHERE id=?`,
		p.Name, p.Months, p.UnitPriceCents, p.PriceCents, p.Status, p.Note, p.ID)
	return err
}

func deleteBillingPackage(id int) error {
	if id <= 0 {
		return errors.New("套餐ID不能为空")
	}
	_, err := db.Exec(`UPDATE billing_packages SET status=0,update_time=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func generateOutTradeNo(userID int) string {
	return fmt.Sprintf("NRL%d%s", userID, time.Now().Format("20060102150405"))
}

func createBillingOrder(u *userinfo, p *billingPackage) (*billingOrder, error) {
	order := &billingOrder{
		OutTradeNo:  generateOutTradeNo(u.ID),
		UserID:      u.ID,
		CallSign:    u.CallSign,
		PackageID:   p.ID,
		Months:      p.Months,
		AmountCents: p.PriceCents,
		Status:      billingStatusNotPay,
	}
	if err := createWechatNativeOrder(order); err != nil {
		return nil, err
	}

	_, err := db.Exec(`INSERT INTO billing_orders(out_trade_no,user_id,callsign,package_id,months,amount_cents,status,prepay_id,code_url,create_time,update_time)
		VALUES(?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		order.OutTradeNo, order.UserID, order.CallSign, order.PackageID, order.Months, order.AmountCents, order.Status, order.PrepayID, order.CodeURL)
	if err != nil {
		return nil, err
	}
	return getBillingOrder(order.OutTradeNo)
}

func getBillingOrder(outTradeNo string) (*billingOrder, error) {
	o := &billingOrder{}
	row := db.QueryRow(`SELECT id,out_trade_no,user_id,callsign,package_id,months,amount_cents,status,
		COALESCE(prepay_id,''),COALESCE(code_url,''),COALESCE(transaction_id,''),COALESCE(payer_openid,''),
		COALESCE(paid_at,''),COALESCE(expire_before,''),COALESCE(expire_after,''),COALESCE(raw_notify,''),create_time,update_time
		FROM billing_orders WHERE out_trade_no=?`, strings.TrimSpace(outTradeNo))
	if err := row.Scan(&o.ID, &o.OutTradeNo, &o.UserID, &o.CallSign, &o.PackageID, &o.Months, &o.AmountCents, &o.Status,
		&o.PrepayID, &o.CodeURL, &o.TransactionID, &o.PayerOpenID, &o.PaidAt, &o.ExpireBefore, &o.ExpireAfter, &o.RawNotify, &o.CreateTime, &o.UpdateTime); err != nil {
		return nil, err
	}
	return o, nil
}

func extendUserExpireTime(userID int, months int) (string, string, error) {
	u, err := getuserByID(userID)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	base := now
	if t, ok := parseAccountExpireTime(u.ExpireTime); ok && t.After(now) {
		base = t
	}
	expireAfter := base.AddDate(0, months, 0).Format("2006-01-02 15:04:05")
	if _, err := db.Exec(`UPDATE users SET expire_time=?,update_time=CURRENT_TIMESTAMP WHERE id=?`, expireAfter, userID); err != nil {
		return "", "", err
	}
	if cached, ok := userlist.Load(u.CallSign); ok {
		cached.(*userinfo).ExpireTime = expireAfter
	}
	return u.ExpireTime, expireAfter, nil
}

func markBillingOrderPaid(outTradeNo, transactionID, payerOpenID, paidAt, raw string) (*billingOrder, error) {
	o, err := getBillingOrder(outTradeNo)
	if err != nil {
		return nil, err
	}
	if o.Status == billingStatusSuccess {
		return o, nil
	}

	expireBefore, expireAfter, err := extendUserExpireTime(o.UserID, o.Months)
	if err != nil {
		return nil, err
	}
	if paidAt == "" {
		paidAt = time.Now().Format("2006-01-02 15:04:05")
	}
	_, err = db.Exec(`UPDATE billing_orders SET status=?,transaction_id=?,payer_openid=?,paid_at=?,expire_before=?,expire_after=?,raw_notify=?,update_time=CURRENT_TIMESTAMP WHERE out_trade_no=?`,
		billingStatusSuccess, transactionID, payerOpenID, paidAt, expireBefore, expireAfter, raw, outTradeNo)
	if err != nil {
		return nil, err
	}
	return getBillingOrder(outTradeNo)
}

func createWechatNativeOrder(order *billingOrder) error {
	if !billingEnabled() {
		return errors.New("收费功能未开启")
	}
	wp := conf.Billing.WechatPay
	notifyURL := strings.TrimSpace(wp.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimSpace(conf.Billing.NotifyURL)
	}
	if wp.AppID == "" || wp.MchID == "" || wp.SerialNo == "" || wp.PrivateKeyPath == "" || notifyURL == "" {
		return errors.New("微信支付配置不完整")
	}

	body := map[string]any{
		"appid":        wp.AppID,
		"mchid":        wp.MchID,
		"description":  firstNonEmpty(wp.Description, "NRL账号续费"),
		"out_trade_no": order.OutTradeNo,
		"notify_url":   notifyURL,
		"amount": map[string]any{
			"total":    order.AmountCents,
			"currency": "CNY",
		},
	}
	payload, _ := json.Marshal(body)
	resp, err := wechatPayRequest("POST", "/v3/pay/transactions/native", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("微信下单失败: %s", string(respBody))
	}

	var result struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}
	if result.CodeURL == "" {
		return errors.New("微信下单未返回二维码地址")
	}
	order.CodeURL = result.CodeURL
	return nil
}

func queryWechatOrder(outTradeNo string) (*billingOrder, error) {
	o, err := getBillingOrder(outTradeNo)
	if err != nil {
		return nil, err
	}
	if o.Status == billingStatusSuccess {
		return o, nil
	}

	path := fmt.Sprintf("/v3/pay/transactions/out-trade-no/%s?mchid=%s", outTradeNo, conf.Billing.WechatPay.MchID)
	resp, err := wechatPayRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("微信查单失败: %s", string(respBody))
	}

	var result struct {
		TradeState    string `json:"trade_state"`
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		SuccessTime   string `json:"success_time"`
		Payer         struct {
			OpenID string `json:"openid"`
		} `json:"payer"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.TradeState == billingStatusSuccess {
		paidAt := result.SuccessTime
		if t, err := time.Parse(time.RFC3339, result.SuccessTime); err == nil {
			paidAt = t.Local().Format("2006-01-02 15:04:05")
		}
		return markBillingOrderPaid(result.OutTradeNo, result.TransactionID, result.Payer.OpenID, paidAt, string(respBody))
	}
	_, _ = db.Exec(`UPDATE billing_orders SET status=?,raw_notify=?,update_time=CURRENT_TIMESTAMP WHERE out_trade_no=?`, result.TradeState, string(respBody), outTradeNo)
	return getBillingOrder(outTradeNo)
}

func wechatPayRequest(method, path string, body []byte) (*http.Response, error) {
	url := "https://api.mch.weixin.qq.com" + path
	reqBody := bytes.NewReader(body)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	auth, err := wechatPayAuthorization(method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	return http.DefaultClient.Do(req)
}

func wechatPayAuthorization(method, canonicalURL string, body []byte) (string, error) {
	nonce, err := randomString(24)
	if err != nil {
		return "", err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := strings.Join([]string{method, canonicalURL, timestamp, nonce, string(body)}, "\n") + "\n"
	signature, err := signWechatPayMessage([]byte(message))
	if err != nil {
		return "", err
	}
	wp := conf.Billing.WechatPay
	return fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		wp.MchID, nonce, signature, timestamp, wp.SerialNo), nil
}

func signWechatPayMessage(message []byte) (string, error) {
	key, err := loadMerchantPrivateKey(conf.Billing.WechatPay.PrivateKeyPath)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func loadMerchantPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("商户私钥解析失败")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("商户私钥不是RSA格式")
	}
	return key, nil
}

func decryptWechatResource(resource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data"`
	Nonce          string `json:"nonce"`
}) ([]byte, error) {
	if resource.Algorithm != "AEAD_AES_256_GCM" {
		return nil, errors.New("不支持的微信回调加密算法")
	}
	key := []byte(conf.Billing.WechatPay.APIv3Key)
	if len(key) != 32 {
		return nil, errors.New("微信支付APIv3Key必须是32字节")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(resource.Ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, []byte(resource.Nonce), ciphertext, []byte(resource.AssociatedData))
}

func (j *jsonapi) httpBillingInfo(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	packages, err := queryBillingPackages(false)
	if err != nil {
		writeJSONResponse(w, &Response{20001, "查询套餐失败", nil})
		return
	}
	writeJSONResponse(w, &Response{20000, "ok", map[string]any{
		"enabled":     billingEnabled(),
		"user":        u,
		"expire_time": u.ExpireTime,
		"packages":    packages,
	}})
}

func (j *jsonapi) httpBillingPackages(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	includeDisabled := checkrole(u, []string{"admin"})
	packages, err := queryBillingPackages(includeDisabled)
	if err != nil {
		writeJSONResponse(w, &Response{20001, "查询套餐失败", nil})
		return
	}
	writeJSONResponseItems(w, packages, len(packages))
}

func (j *jsonapi) httpBillingPackageCreate(w http.ResponseWriter, req *http.Request) {
	j.handleBillingPackageWrite(w, req, "create")
}

func (j *jsonapi) httpBillingPackageUpdate(w http.ResponseWriter, req *http.Request) {
	j.handleBillingPackageWrite(w, req, "update")
}

func (j *jsonapi) httpBillingPackageDelete(w http.ResponseWriter, req *http.Request) {
	j.handleBillingPackageWrite(w, req, "delete")
}

func (j *jsonapi) handleBillingPackageWrite(w http.ResponseWriter, req *http.Request, action string) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()
	p := &billingPackage{}
	if err := jsonextra.Unmarshal(body, p); err != nil {
		writeJSONResponse(w, &Response{20001, "套餐参数错误", nil})
		return
	}
	switch action {
	case "create":
		err = createBillingPackage(p)
	case "update":
		err = updateBillingPackage(p)
	case "delete":
		err = deleteBillingPackage(p.ID)
	}
	if err != nil {
		writeJSONResponse(w, &Response{20001, err.Error(), nil})
		return
	}
	addOperatorLog(string(body), "收费套餐"+action, u)
	writeJSONResponse(w, &Response{20000, "操作成功", nil})
}

func (j *jsonapi) httpBillingOrderCreate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !billingEnabled() {
		writeJSONResponse(w, &Response{20001, "收费功能未开启", nil})
		return
	}
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()
	reqData := &billingOrderReq{}
	if err := jsonextra.Unmarshal(body, reqData); err != nil {
		writeJSONResponse(w, &Response{20001, "订单参数错误", nil})
		return
	}
	p, err := getBillingPackage(reqData.PackageID)
	if err != nil || p.Status != 1 {
		writeJSONResponse(w, &Response{20001, "套餐不存在或已停用", nil})
		return
	}
	order, err := createBillingOrder(u, p)
	if err != nil {
		writeJSONResponse(w, &Response{20001, err.Error(), nil})
		return
	}
	writeJSONResponse(w, &Response{20000, "ok", order})
}

func (j *jsonapi) httpBillingOrderQuery(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()
	reqData := &billingOrderReq{}
	if err := jsonextra.Unmarshal(body, reqData); err != nil {
		writeJSONResponse(w, &Response{20001, "订单参数错误", nil})
		return
	}
	order, err := getBillingOrder(reqData.OutTradeNo)
	if err != nil {
		writeJSONResponse(w, &Response{20001, "订单不存在", nil})
		return
	}
	if order.UserID != u.ID && !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}
	if order.Status != billingStatusSuccess && billingEnabled() {
		if refreshed, err := queryWechatOrder(order.OutTradeNo); err == nil {
			order = refreshed
		}
	}
	writeJSONResponse(w, &Response{20000, "ok", order})
}

func (j *jsonapi) httpBillingWechatNotify(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	body, _ := io.ReadAll(req.Body)
	req.Body.Close()
	var notify struct {
		Resource struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &notify); err != nil {
		writeWechatNotifyResult(w, "FAIL", "回调参数错误")
		return
	}
	plain, err := decryptWechatResource(notify.Resource)
	if err != nil {
		log.Println("billing notify decrypt failed:", err)
		writeWechatNotifyResult(w, "FAIL", "回调解密失败")
		return
	}
	var tx struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		SuccessTime   string `json:"success_time"`
		Payer         struct {
			OpenID string `json:"openid"`
		} `json:"payer"`
	}
	if err := json.Unmarshal(plain, &tx); err != nil {
		writeWechatNotifyResult(w, "FAIL", "交易参数错误")
		return
	}
	if tx.TradeState == billingStatusSuccess {
		paidAt := tx.SuccessTime
		if t, err := time.Parse(time.RFC3339, tx.SuccessTime); err == nil {
			paidAt = t.Local().Format("2006-01-02 15:04:05")
		}
		if _, err := markBillingOrderPaid(tx.OutTradeNo, tx.TransactionID, tx.Payer.OpenID, paidAt, string(plain)); err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Println("billing mark paid failed:", err)
			writeWechatNotifyResult(w, "FAIL", "订单处理失败")
			return
		}
	}
	writeWechatNotifyResult(w, "SUCCESS", "成功")
}

func writeWechatNotifyResult(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func randomString(n int) (string, error) {
	const letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}
