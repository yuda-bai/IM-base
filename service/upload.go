package service

import (
	"fmt"
	"ginchat/common"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

func UploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		common.Fail(c, "请选择图片")
		return
	}
	allowedExts := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
	}
	ext := filepath.Ext(file.Filename)
	if !allowedExts[ext] {
		common.Fail(c, "请上传正确的图片格式")
		return
	}
	if file.Size > 1024*1024*5 {
		common.Fail(c, "图片大小不能超过5M")
		return
	}
	uploadDir := "uploads/" // 图片上传目录
	err = os.MkdirAll(uploadDir, os.ModePerm)
	if err != nil {
		return
	}

	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		fmt.Println("保存图片失败:", err)
		common.Fail(c, "图片上传失败")
		return
	}
	imagUrl := "/uploads/" + filename
	common.Success(c, "图片上传成功", map[string]string{
		"imagUrl": imagUrl,
	})
}
