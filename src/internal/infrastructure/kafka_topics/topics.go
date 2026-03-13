package kafka_topics

type Topic string

const (
	UserRegisterTopic      Topic = "user-register-v1"
	UserRegisterErrorTopic Topic = "error-user-register-v1"
	UserLoginTopic         Topic = "user-login-v1"
	SystemLogsTopic        Topic = "system-logs-v1"
	ChatMessagesTopic      Topic = "messages-v1"
	LogoutToipic           Topic = "logout-v1"
	ChangeProfile          Topic = "change-profile-v1"
	ChangeProfileAvatar    Topic = "change-profile-avatar-v1"
	SearchUser             Topic = "search-user-v1"
	NewActionProfile       Topic = "new-action-profile-v1"
	ActionLikeProfile      Topic = "action-like-profile-v1"
	MatchFind              Topic = "match-find-v1"
	MatchFindNotification  Topic = "match-notification-v1"
	LikeNotification       Topic = "like-notification-v1"
)

// String преобразует тип Topic в строку для библиотек
func (t Topic) String() string {
	return string(t)
}
