package main

import "encoding/json"

type Erron[T any] struct {
	Errno     int         `json:"errno"`
	RequestID json.Number `json:"request_id"`
	Data      T           `json:"data"`
}

type WXListResult struct {
	List []WXListFile `json:"list"`
	More bool         `json:"has_more"`

	Uk      json.Number `json:"uk"`
	Shareid json.Number `json:"shareid"`
	Seckey  string      `json:"seckey"`
}

type WXListFile struct {
	Fsid   json.Number       `json:"fs_id"`
	Isdir  json.Number       `json:"isdir"`
	Path   string            `json:"path"`
	Name   string            `json:"server_filename"`
	Mtime  json.Number       `json:"server_mtime"`
	Ctime  json.Number       `json:"server_ctime"`
	Size   json.Number       `json:"size"`
	Md5    string            `json:"md5"`
	Thumbs map[string]string `json:"thumbs"`
}

// DownloadLinkResult 用于解析 sharedownload 接口的 JSON 响应
type DownloadLinkResult struct {
	Errno int                `json:"errno"`
	List  []DownloadListItem `json:"list"`
}

type DownloadListItem struct {
	Dlink string `json:"dlink"`
}
