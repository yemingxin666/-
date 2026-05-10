package oss

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import "github.com/gin-gonic/gin"

const Local = "local"
const Minio = "minio"
const QiNiu = "qiniu"
const AliYun = "aliyun"

type File struct {
	Name   string `json:"name"`
	ObjKey string `json:"obj_key"`
	Size   int64  `json:"size"`
	URL    string `json:"url"`
	Ext    string `json:"ext"`
}
type Uploader interface {
	PutFile(ctx *gin.Context, name string) (File, error)
	PutUrlFile(url string, ext string, useProxy bool) (string, error)
	PutBase64(imageData string) (string, error)
	// PutBytes 直接上传已在内存中的字节数据。
	// ext 不含点（如 "png"/"jpg"）也不要求含点；空值时按 .png 处理。
	// 返回最终可访问的文件 URL。
	PutBytes(data []byte, ext string) (string, error)
	Delete(fileURL string) error
	// SignURL 对已存储的文件 URL 生成带时效的访问地址；不支持签名的实现直接返回原 URL
	SignURL(fileURL string, expireSeconds int64) (string, error)
}

// normalizeExt 统一扩展名格式：空串返回 ".png"；否则确保以 "." 开头。
// 仅 oss 内部使用，各实现共用。
func normalizeExt(ext string) string {
	if ext == "" {
		return ".png"
	}
	if ext[0] != '.' {
		return "." + ext
	}
	return ext
}

// mimeByExt 根据扩展名推断 Content-Type，未知类型回退为 application/octet-stream。
func mimeByExt(ext string) string {
	switch normalizeExt(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
