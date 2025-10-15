package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	openlistwasiplugindriver "github.com/OpenListTeam/openlist-wasi-plugin-driver"
	drivertypes "github.com/OpenListTeam/openlist-wasi-plugin-driver/binding/openlist/plugin-driver/types"
	"github.com/pkg/errors"
)

// calculateRand 函数逻辑不变, 它依赖的组件 (rc4, sha1) 均已兼容
func (bs *BaiduShare) calculateRand(timestamp int64) (string, error) {
	if bs.info.Bduss == "" || bs.info.Uid == "" || bs.info.Sk == "" {
		return "", fmt.Errorf("missing personal info (BDUSS, UID, SK) to calculate rand")
	}
	trueSK, err := getTrueSK(bs.info.Sk, bs.info.Uid)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt true SK: %w", err)
	}

	sha1HashOfBduss := sha1.Sum([]byte(bs.info.Bduss))
	sha1OfBdussHex := hex.EncodeToString(sha1HashOfBduss[:])
	var baseBuilder strings.Builder
	baseBuilder.WriteString(sha1OfBdussHex)
	baseBuilder.WriteString(bs.info.Uid)
	baseBuilder.WriteString(trueSK)
	baseBuilder.WriteString(strconv.FormatInt(timestamp, 10))
	baseBuilder.WriteString(deviceID)
	baseBuilder.WriteString(appVersion)
	baseBuilder.WriteString(appSignatureCertMD5)
	finalHash := sha1.Sum([]byte(baseBuilder.String()))
	return hex.EncodeToString(finalHash[:]), nil
}

func (bs *BaiduShare) initShareInfo(ctx context.Context) error {
	openlistwasiplugindriver.Debugf("Attempting to initialize share info: %s", bs.Surl)

	var result Erron[WXListResult]

	// Prepare request data
	formData := map[string]string{
		"pwd":      bs.Pwd,
		"root":     "1",
		"shorturl": bs.Surl,
	}

	// Make the API request
	resp, err := bs.Client.R().
		SetContext(ctx).
		SetFormData(formData).
		SetResult(&result).
		Post("share/wxlist?channel=weixin&version=2.2.2&clienttype=25&web=1")

	// Handle HTTP client-level errors (e.g., network issues)
	if err != nil {
		openlistwasiplugindriver.Errorf("HTTP request execution failed: %s", err)
		return errors.Errorf("could not execute request for surl %s: %w", bs.Surl, err)
	}

	// Handle API-level errors or unexpected status codes
	if resp.StatusCode() != http.StatusOK || result.Errno != 0 {
		openlistwasiplugindriver.Errorf("API returned an error: status_code => %d, errno => %d, response_body => %s", resp.StatusCode(), result.Errno, resp.String())
		return errors.Errorf("API error for surl %s: status_code=%d, errno=%d", bs.Surl, resp.StatusCode(), result.Errno)
	}

	// Ensure the file list is not empty to prevent a panic
	if len(result.Data.List) == 0 {
		openlistwasiplugindriver.Warnf("API response successful but file list is empty: %s", bs.Surl)
		return errors.New("share content is empty or inaccessible")
	}

	// Populate the info struct on success
	bs.info.Root = path.Dir(result.Data.List[0].Path)
	bs.info.Seckey = DecodeSceKey(result.Data.Seckey)
	bs.info.Shareid = result.Data.Shareid.String()
	bs.info.Uk = result.Data.Uk.String()
	openlistwasiplugindriver.Debugf("Successfully initialized share info: shareid => %s, uk => %s, root => %s", bs.info.Shareid, bs.info.Uk, bs.info.Root)
	return nil
}

func (bs *BaiduShare) initUidAndSk(ctx context.Context) error {
	// 步骤 1: 从 Cookie 字符串中提取 BDUSS
	re := regexp.MustCompile(`BDUSS=([^;]+)`)
	matches := re.FindStringSubmatch(bs.Cookie)
	if len(matches) < 2 {
		return errors.New("BDUSS not found in cookies")
	}
	bs.info.Bduss = matches[1]

	// 步骤 2: 调用 Tieba Sync API 获取 UID
	openlistwasiplugindriver.Debugf("[*] Attempting to get user UID via Tieba sync API...")

	type SyncData struct {
		UserID json.Number `json:"user_id"`
	}
	var syncResult Erron[SyncData]

	resp, err := bs.Client.R().
		SetContext(ctx).
		SetResult(&syncResult).
		Get("https://tieba.baidu.com/mo/q/sync")

	// 检查请求是否成功
	if err != nil {
		return errors.Wrap(err, "error calling Tieba sync API")
	}

	if resp.StatusCode() != http.StatusOK || syncResult.Errno != 0 || syncResult.Data.UserID == "" {
		return errors.Errorf("failed to get UID from sync API: (status: %d, errno: %d)", resp.StatusCode(), syncResult.Errno)
	}

	uid := syncResult.Data.UserID.String()
	bs.info.Uid = uid
	openlistwasiplugindriver.Debugf("[*] Successfully obtained UID via sync API: %s\n", uid)

	// 步骤 3: 使用获取到的 UID 获取加密的 SK
	openlistwasiplugindriver.Debugf("[*] Attempting to get encrypted SK...")

	type SkResult struct {
		Errno int    `json:"errno"`
		Uinfo string `json:"uinfo"` // 加密的 SK
	}
	var skResult SkResult

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	resp, err = bs.Client.R().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"action":     "ANDROID_ACTIVE_BACKGROUND_UPLOAD_AND_DOWNLOAD",
			"clienttype": "1",
			"needrookie": "1",
			"timestamp":  timestamp,
			"bind_uid":   uid,
			"channel":    "android",
		}).
		SetResult(&skResult).
		Get("https://pan.baidu.com/api/report/user")

	if err != nil {
		return errors.Wrap(err, "error calling get SK API")
	}

	if resp.StatusCode() == http.StatusOK && skResult.Errno == 0 && skResult.Uinfo != "" {
		bs.info.Sk = skResult.Uinfo
		openlistwasiplugindriver.Debugf("[*] Successfully obtained encrypted SK: %s\n", bs.info.Sk)
	} else {
		return errors.Errorf("failed to get SK: (status: %d, errno: %d)", resp.StatusCode(), skResult.Errno)
	}

	return nil
}

func (bs *BaiduShare) GetDownloadLink(ctx context.Context, fileInfo drivertypes.Object) (string, error) {
	openlistwasiplugindriver.Debugf("[*] Attempting to get download link for: %s\n", fileInfo.Name)

	timestamp := time.Now().Unix()
	timestampMs := timestamp * 1000

	// 1. 计算 rand
	rand, err := bs.calculateRand(timestampMs)
	if err != nil {
		return "", err
	}

	// 2. 构造 POST Body (恢复使用 encoding/json)
	extraData := map[string]string{"sekey": bs.info.Seckey}
	extraJSON, err := json.Marshal(extraData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal extra field: %w", err)
	}
	postBodyString := fmt.Sprintf(
		"encrypt=0&uk=%s&product=share&primaryid=%s&fid_list=[\"%s\"]&extra=%s",
		bs.info.Uk, bs.info.Shareid, fileInfo.ID, string(extraJSON),
	)

	// 3. 计算 sign
	sign := generateSharedownloadSign(postBodyString, timestamp)

	// 4. 准备 Query Params
	queryParams := map[string]string{
		"sign": sign, "timestamp": strconv.FormatInt(timestamp, 10), "rand": rand,
		"time": strconv.FormatInt(timestampMs, 10), "devuid": deviceID,
		"channel": "android", "clienttype": "1", "version": appVersion,
	}

	var result DownloadLinkResult
	resp, err := bs.Client.R().
		SetContext(ctx).
		SetQueryParams(queryParams).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetBody(postBodyString).
		SetResult(&result).
		Post("api/sharedownload")

	if err != nil {
		return "", fmt.Errorf("request execution failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || result.Errno != 0 {
		return "", fmt.Errorf("api error: status_code=%d, errno=%d, body=%s", resp.StatusCode(), result.Errno, resp.String())
	}

	if len(result.List) == 0 || result.List[0].Dlink == "" {
		return "", fmt.Errorf("dlink not found in response: %s", resp.String())
	}

	dlink := result.List[0].Dlink
	openlistwasiplugindriver.Debugf("[*] Successfully retrieved download link for: %s\n", fileInfo.Name)
	return dlink, nil
}
