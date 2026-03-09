package startflags

type Flag string

const (
	UserRegisterFlag    Flag = "user_register"
	UserLoginFlag       Flag = "user_login"
	UserChangePassword  Flag = "user_change_password"
	SendMessage         Flag = "send_message"
	GetMessages         Flag = "get_messages"
	ChangeProfile       Flag = "change_profile"
	ChangeProfileAvatar Flag = "change_profile_avatar"
	SearchUserFlag      Flag = "search_user"
)
