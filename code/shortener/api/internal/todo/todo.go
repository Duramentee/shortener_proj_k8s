// Package todo 只提供一件事：生成「尚未实现」的错误信息。
//
// 本包存在的目的是让整个工程在逐步填写的过程中始终可以编译、可以启动。
// 未填写的处理函数会返回 HTTP 状态码 501，日志里会写出对应的任务分组编号，
// 因此你可以一边填写一边运行、一边用 curl 验证，不必等到全部写完才能看到结果。
//
// 当任务清单中的全部分组完成之后，本包以及所有引用它的位置都可以整体删除。
package todo

import (
	"errors"
	"fmt"
)

// Message 生成一条指明函数名与任务分组的提示信息。
// 参数 name 是「包名.函数名」形式的位置，参数 group 是任务清单中的分组编号。
func Message(name string, group int) string {
	return fmt.Sprintf("尚未实现：%s，请完成任务清单的第 %d 组任务", name, group)
}

// Error 把 Message 的取值包装成一个错误，供返回 error 的函数使用。
func Error(name string, group int) error {
	return errors.New(Message(name, group))
}
