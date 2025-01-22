//版权所有 © 2023 OpenIM 开源社区。版权所有。
//
//根据 Apache 许可证 2.0 版（“许可证”）获得许可；
//除非遵守许可证，否则您不得使用此文件。
//您可以在以下位置获取许可证副本：
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//除非适用法律要求或书面同意，否则软件
//根据许可证分发是在“按原样”基础上分发的，
//不提供任何明示或暗示的保证或条件。
//请参阅许可证以了解特定语言的管理权限和
//许可证下的限制。

package mw

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/openimsdk/chat/pkg/common/constant"
	"github.com/openimsdk/chat/pkg/protocol/admin"
	"github.com/openimsdk/tools/apiresp"
	"github.com/openimsdk/tools/errs"
)

func New(client admin.AdminClient) *MW {
	return &MW{client: client}
}

type MW struct {
	client admin.AdminClient
}

func (o *MW) parseToken(c *gin.Context) (string, int32, string, error) {
	token := c.GetHeader("token")
	if token == "" {
		return "", 0, "", errs.ErrArgs.WrapMsg("token is empty")
	}
	resp, err := o.client.ParseToken(c, &admin.ParseTokenReq{Token: token})
	if err != nil {
		return "", 0, "", err
	}
	return resp.UserID, resp.UserType, token, nil
}

func (o *MW) parseTokenType(c *gin.Context, userType int32) (string, string, error) {
	userID, t, token, err := o.parseToken(c)
	if err != nil {
		return "", "", err
	}
	if t != userType {
		return "", "", errs.ErrArgs.WrapMsg("token type error")
	}
	return userID, token, nil
}

func (o *MW) setToken(c *gin.Context, userID string, userType int32) {
	SetToken(c, userID, userType)
}

func (o *MW) CheckToken(c *gin.Context) {
	userID, userType, _, err := o.parseToken(c)
	if err != nil {
		c.Abort()
		apiresp.GinError(c, err)
		return
	}
	o.setToken(c, userID, userType)
}

func (o *MW) CheckAdmin(c *gin.Context) {
	userID, _, err := o.parseTokenType(c, constant.AdminUser)
	if err != nil {
		c.Abort()
		apiresp.GinError(c, err)
		return
	}
	o.setToken(c, userID, constant.AdminUser)
}

func (o *MW) CheckUser(c *gin.Context) {
	userID, _, err := o.parseTokenType(c, constant.NormalUser)
	if err != nil {
		c.Abort()
		apiresp.GinError(c, err)
		return
	}
	o.setToken(c, userID, constant.NormalUser)
}

func (o *MW) CheckAdminOrNil(c *gin.Context) {
	defer c.Next()
	userID, userType, _, err := o.parseToken(c)
	if err != nil {
		return
	}
	if userType == constant.AdminUser {
		o.setToken(c, userID, constant.AdminUser)
	}
}

func SetToken(c *gin.Context, userID string, userType int32) {
	c.Set(constant.RpcOpUserID, userID)
	c.Set(constant.RpcOpUserType, []string{strconv.Itoa(int(userType))})
	c.Set(constant.RpcCustomHeader, []string{constant.RpcOpUserType})
}
