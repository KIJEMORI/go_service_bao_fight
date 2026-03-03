package kafka_topics

type Topic string

const (
	UserRegisterTopic      Topic = "user-register-v1"
	UserRegisterErrorTopic Topic = "error-user-register-v1"
	UserLoginTopic         Topic = "user-login-v1"
	SystemLogsTopic        Topic = "system-logs-v1"
	ChatMessagesTopic      Topic = "messages-v1"
	LogoutToipic           Topic = "logout-v1"
)

// String преобразует тип Topic в строку для библиотек
func (t Topic) String() string {
	return string(t)
}
