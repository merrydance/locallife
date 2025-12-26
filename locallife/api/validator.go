package api

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/merrydance/locallife/val"
)

// registerCustomValidators 注册自定义验证器
func registerCustomValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// 注册身份证验证器
		v.RegisterValidation("validIDCard", validIDCard)
		// 注册手机号验证器
		v.RegisterValidation("validPhone", validPhone)
	}
}

// validIDCard 验证身份证号格式和校验码
var validIDCard validator.Func = func(fl validator.FieldLevel) bool {
	idCard, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return val.ValidateIDCard(idCard) == nil
}

// validPhone 验证中国大陆手机号格式
var validPhone validator.Func = func(fl validator.FieldLevel) bool {
	phone, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return val.ValidatePhone(phone) == nil
}
