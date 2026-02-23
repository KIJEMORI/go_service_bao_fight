package kafka

type Topic string

const (
	UserRegisterTopic Topic = "user-register-v1"
	UserLoginTopic    Topic = "user-login-v1"
	SystemLogsTopic   Topic = "system-logs-v1"
)

// String преобразует тип Topic в строку для библиотек
func (t Topic) String() string {
	return string(t)
}
