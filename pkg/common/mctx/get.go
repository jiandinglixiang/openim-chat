// Copyright © 2023 OpenIM open source community. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mctx

import (
	"context"
	"github.com/openimsdk/tools/utils/datautil"
	"strconv"

	constantpb "github.com/openimsdk/protocol/constant"
	"github.com/openimsdk/tools/errs"

	"github.com/openimsdk/chat/pkg/common/constant"
	"github.com/openimsdk/chat/pkg/common/tokenverify"
)

func HaveOpUser(ctx context.Context) bool {
	return ctx.Value(constant.RpcOpUserID) != nil
}

func Check(ctx context.Context) (string, int32, error) {
	opUserIDVal := ctx.Value(constant.RpcOpUserID) // 从上下文中获取操作用户ID的值
	opUserID, ok := opUserIDVal.(string)           // 将获取到的值转换为字符串类型
	if !ok {                                       // 如果转换失败
		return "", 0, errs.ErrNoPermission.WrapMsg("no opUserID") // 返回错误信息，表示没有操作用户ID
	}
	if opUserID == "" { // 如果操作用户ID为空
		return "", 0, errs.ErrNoPermission.WrapMsg("opUserID empty") // 返回错误信息，表示操作用户ID为空
	}
	opUserTypeArr, ok := ctx.Value(constant.RpcOpUserType).([]string) // 从上下文中获取操作用户类型的值，并转换为字符串切片
	if !ok {                                                          // 如果转换失败
		return "", 0, errs.ErrNoPermission.WrapMsg("missing user type") // 返回错误信息，表示缺少用户类型
	}
	if len(opUserTypeArr) == 0 { // 如果用户类型数组为空
		return "", 0, errs.ErrNoPermission.WrapMsg("user type empty") // 返回错误信息，表示用户类型为空
	}
	userType, err := strconv.Atoi(opUserTypeArr[0]) // 将用户类型的第一个元素转换为整数
	if err != nil {                                 // 如果转换失败
		return "", 0, errs.ErrNoPermission.WrapMsg("user type invalid " + err.Error()) // 返回错误信息，表示用户类型无效，并附加错误信息
	}
	if !(userType == constant.AdminUser || userType == constant.NormalUser) { // 如果用户类型既不是管理员也不是普通用户
		return "", 0, errs.ErrNoPermission.WrapMsg("user type invalid") // 返回错误信息，表示用户类型无效
	}
	return opUserID, int32(userType), nil // 返回操作用户ID和用户类型
}

func CheckAdmin(ctx context.Context) (string, error) {
	userID, userType, err := Check(ctx)
	if err != nil {
		return "", err
	}
	if userType != constant.AdminUser {
		return "", errs.ErrNoPermission.WrapMsg("not admin")
	}
	return userID, nil
}

func CheckUser(ctx context.Context) (string, error) {
	userID, userType, err := Check(ctx)
	if err != nil {
		return "", err
	}
	if userType != constant.NormalUser {
		return "", errs.ErrNoPermission.WrapMsg("not user")
	}
	return userID, nil
}

func CheckAdminOrUser(ctx context.Context) (string, int32, error) {
	userID, userType, err := Check(ctx)
	if err != nil {
		return "", 0, err
	}
	return userID, userType, nil
}

func CheckAdminOr(ctx context.Context, userIDs ...string) error {
	userID, userType, err := Check(ctx)
	if err != nil {
		return err
	}
	if userType == tokenverify.TokenAdmin {
		return nil
	}
	for _, id := range userIDs {
		if userID == id {
			return nil
		}
	}
	return errs.ErrNoPermission.WrapMsg("not admin or not in userIDs")
}

func GetOpUserID(ctx context.Context) string {
	userID, _ := ctx.Value(constantpb.OpUserID).(string)
	return userID
}

func GetUserType(ctx context.Context) (int, error) {
	userTypeArr, _ := ctx.Value(constant.RpcOpUserType).([]string)
	userType, err := strconv.Atoi(userTypeArr[0])
	if err != nil {
		return 0, errs.ErrNoPermission.WrapMsg("user type invalid " + err.Error())
	}
	return userType, nil
}

func WithOpUserID(ctx context.Context, opUserID string, userType int) context.Context {
	headers, _ := ctx.Value(constant.RpcCustomHeader).([]string)
	ctx = context.WithValue(ctx, constant.RpcOpUserID, opUserID)
	ctx = context.WithValue(ctx, constant.RpcOpUserType, []string{strconv.Itoa(userType)})
	if datautil.IndexOf(constant.RpcOpUserType, headers...) < 0 {
		ctx = context.WithValue(ctx, constant.RpcCustomHeader, append(headers, constant.RpcOpUserType))
	}
	return ctx
}

func WithAdminUser(ctx context.Context, userID string) context.Context {
	return WithOpUserID(ctx, userID, constant.AdminUser)
}

func WithApiToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, constant.CtxApiToken, token)
}
