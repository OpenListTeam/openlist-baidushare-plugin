package main

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strconv"
	"time"

	openlistwasiplugindriver "github.com/OpenListTeam/openlist-wasi-plugin-driver"
	drivertypes "github.com/OpenListTeam/openlist-wasi-plugin-driver/binding/openlist/plugin-driver/types"
	httptypes "github.com/OpenListTeam/openlist-wasi-plugin-driver/binding/wasi/http/types"
	"resty.dev/v3"
	"resty.dev/v3/cookiejar"

	"go.bytecodealliance.org/cm"
)

func init() {
	openlistwasiplugindriver.RegisterDriver(&BaiduShare{})
}

type Addition struct {
	openlistwasiplugindriver.RootPath
	Surl   string `json:"surl"`
	Pwd    string `json:"pwd"`
	Cookie string `json:"cookie"`
}

var _ openlistwasiplugindriver.Driver = (*BaiduShare)(nil)

type BaiduShare struct {
	openlistwasiplugindriver.DriverHandle
	Addition

	Client *resty.Client

	info struct {
		Root    string
		Seckey  string
		Shareid string
		Uk      string

		Bduss string
		Uid   string
		Sk    string
	}
}

func (*BaiduShare) GetProperties() drivertypes.DriverProps {
	return drivertypes.DriverProps{
		Name:        "BaiduShare",
		Capabilitys: drivertypes.CapabilityListFile | drivertypes.CapabilityLinkFile,
	}
}

func (*BaiduShare) GetFormMeta() []drivertypes.FormField {
	return []drivertypes.FormField{
		{
			Name:     "cookie",
			Label:    "Cookie",
			Kind:     drivertypes.FieldKindPasswordKind(""),
			Required: true,
			Help:     "",
		},
		{
			Name:     "surl",
			Label:    "surl",
			Kind:     drivertypes.FieldKindStringKind(""),
			Required: true,
			Help:     "",
		},
		{
			Name:     "pwd",
			Label:    "pwd",
			Kind:     drivertypes.FieldKindStringKind(""),
			Required: false,
			Help:     "",
		},
		{
			Name:     "root_folder_path",
			Label:    "rootFolderPath",
			Kind:     drivertypes.FieldKindStringKind(""),
			Required: false,
			Help:     "",
		},
	}
}

func (bs *BaiduShare) Init(ctx context.Context) error {
	if err := bs.LoadConfig(&bs.Addition); err != nil {
		return err
	}
	cookies, err := cookiejar.ParseCookie(bs.Cookie)
	if err != nil {
		return err
	}

	bs.Client = resty.New().SetBaseURL("https://pan.baidu.com/").SetHeader("User-Agent", "netdisk")

	u, _ := url.Parse("https://baidu.com/")
	for _, cookie := range cookies {
		cookie.Domain = "baidu.com"
		cookie.Path = "/"
	}
	bs.Client.CookieJar().SetCookies(u, cookies)
	if err := bs.initUidAndSk(ctx); err != nil {
		return err
	}

	if err := bs.initShareInfo(ctx); err != nil {
		return err
	}

	return nil
}

func (bs *BaiduShare) Drop(ctx context.Context) error {
	bs.Client = nil
	if err := bs.SaveConfig(&bs.Addition); err != nil {
		return err
	}
	return nil
}

func (bs *BaiduShare) ListFiles(ctx context.Context, dir drivertypes.Object) ([]drivertypes.Object, error) {
	reqDir := dir.Path
	isRoot := "0"
	if reqDir == bs.RootFolderPath {
		reqDir = path.Join(bs.info.Root, reqDir)
	}
	if reqDir == bs.info.Root {
		isRoot = "1"
	}

	objs := make([]drivertypes.Object, 0)
	var page uint64 = 1
	more := true
	for more {
		var respJson Erron[WXListResult]
		resp, err := bs.Client.R().
			SetContext(ctx).
			SetFormData(map[string]string{
				"dir":      reqDir,
				"num":      "1000",
				"order":    "time",
				"page":     strconv.FormatUint(page, 10),
				"pwd":      bs.Pwd,
				"root":     isRoot,
				"shorturl": bs.Surl,
			}).
			SetResult(&respJson).
			Post("share/wxlist?channel=weixin&version=2.2.2&clienttype=25&web=1")
		if err != nil {
			return nil, err
		}

		if resp.StatusCode() != 200 || respJson.Errno != 0 {
			return nil, errors.New("http_status: " + strconv.Itoa(resp.StatusCode()) + "; http_body: " + resp.String())
		}

		page++
		more = respJson.Data.More
		for _, v := range respJson.Data.List {
			size, _ := v.Size.Int64()
			mtime, _ := v.Mtime.Int64()
			ctime, _ := v.Ctime.Int64()

			var thumb cm.Option[string]
			if th, err := ReplaceSizeAndQuality(v.Thumbs["icon"], 800, 800, 100); err != nil {
				thumb = cm.None[string]()
			} else {
				thumb = cm.Some(th)
			}

			objs = append(objs, drivertypes.Object{
				ID:       v.Fsid.String(),
				Path:     v.Path,
				Name:     v.Name,
				Size:     size,
				Modified: drivertypes.Duration(time.Unix(mtime, 0).UnixNano()),
				Created:  drivertypes.Duration(time.Unix(ctime, 0).UnixNano()),
				IsFolder: v.Isdir.String() == "1",
				Thumb:    thumb,
				Hashes: cm.ToList([]drivertypes.HashInfo{
					{
						Alg: drivertypes.HashAlgMd5,
						Val: DecryptMd5(v.Md5),
					},
				}),
			})
		}
	}
	return objs, nil
}

func (bs *BaiduShare) LinkFile(ctx context.Context, file drivertypes.Object, args drivertypes.LinkArgs) (*drivertypes.LinkResource, *drivertypes.Object, error) {
	dlink, err := bs.GetDownloadLink(ctx, file)
	if err != nil {
		return nil, nil, err
	}

	header := httptypes.NewFields()
	header.Append("User-Agent", httptypes.FieldValue(cm.ToList([]byte("netdisk"))))

	link := drivertypes.LinkResourceDirect(drivertypes.LinkInfo{
		URL:     dlink,
		Headers: header,
	})
	return &link, nil, nil
}
