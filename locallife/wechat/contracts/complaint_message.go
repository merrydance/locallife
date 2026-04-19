package contracts

const (
	ComplaintMessageBlockTypeText        = "TEXT"
	ComplaintMessageBlockTypeImage       = "IMAGE"
	ComplaintMessageBlockTypeLink        = "LINK"
	ComplaintMessageBlockTypeFAQList     = "FAQ_LIST"
	ComplaintMessageBlockTypeButton      = "BUTTON"
	ComplaintMessageBlockTypeButtonGroup = "BUTTON_GROUP"
)

const (
	ComplaintMessageTextColorDefault   = "DEFAULT"
	ComplaintMessageTextColorSecondary = "SECONDARY"
)

const (
	ComplaintMessageImageStyleTypeNarrow = "IMAGE_STYLE_TYPE_NARROW"
	ComplaintMessageImageStyleTypeWide   = "IMAGE_STYLE_TYPE_WIDE"
)

const (
	ComplaintMessageActionTypeSendMessage = "ACTION_TYPE_SEND_MESSAGE"
	ComplaintMessageActionTypeJumpURL     = "ACTION_TYPE_JUMP_URL"
	ComplaintMessageActionTypeJumpMiniApp = "ACTION_TYPE_JUMP_MINI_PROGRAM"
)

const (
	ComplaintMessageButtonLayoutUnknown    = "LAYOUT_UNKNOWN"
	ComplaintMessageButtonLayoutHorizontal = "LAYOUT_HORIZONTAL"
	ComplaintMessageButtonLayoutVertical   = "LAYOUT_VERTICAL"
)

const (
	ComplaintMessageSenderIdentityUnknown = "UNKNOWN"
	ComplaintMessageSenderIdentityManual  = "MANUAL"
	ComplaintMessageSenderIdentityMachine = "MACHINE"
)

type ComplaintNormalMessage struct {
	Blocks         []ComplaintMessageBlock `json:"blocks,omitempty"`
	SenderIdentity string                  `json:"sender_identity,omitempty"`
	CustomData     string                  `json:"custom_data,omitempty"`
}

type ComplaintClickMessage struct {
	MessageContent string `json:"message_content,omitempty"`
	ActionID       string `json:"action_id,omitempty"`
	ClickedLogID   string `json:"clicked_log_id,omitempty"`
}

type ComplaintMessageBlock struct {
	Type        string                       `json:"type,omitempty"`
	Text        *ComplaintTextMessage        `json:"text,omitempty"`
	Image       *ComplaintImageMessage       `json:"image,omitempty"`
	Link        *ComplaintLinkMessage        `json:"link,omitempty"`
	FAQList     *ComplaintFAQListMessage     `json:"faq_list,omitempty"`
	Button      *ComplaintButtonMessage      `json:"button,omitempty"`
	ButtonGroup *ComplaintButtonGroupMessage `json:"button_group,omitempty"`
}

type ComplaintTextMessage struct {
	Text   string `json:"text,omitempty"`
	Color  string `json:"color,omitempty"`
	IsBold bool   `json:"is_bold,omitempty"`
}

type ComplaintImageMessage struct {
	MediaID        string `json:"media_id,omitempty"`
	ImageStyleType string `json:"image_style_type,omitempty"`
}

type ComplaintLinkMessage struct {
	Text        string                  `json:"text,omitempty"`
	Action      *ComplaintMessageAction `json:"action,omitempty"`
	InvalidInfo *ComplaintInvalidInfo   `json:"invalid_info,omitempty"`
}

type ComplaintFAQListMessage struct {
	FAQs []ComplaintFAQItem `json:"faqs,omitempty"`
}

type ComplaintButtonMessage struct {
	Text        string                  `json:"text,omitempty"`
	Action      *ComplaintMessageAction `json:"action,omitempty"`
	InvalidInfo *ComplaintInvalidInfo   `json:"invalid_info,omitempty"`
}

type ComplaintButtonGroupMessage struct {
	Buttons      []ComplaintInnerButton `json:"buttons,omitempty"`
	ButtonLayout string                 `json:"button_layout,omitempty"`
	InvalidInfo  *ComplaintInvalidInfo  `json:"invalid_info,omitempty"`
}

type ComplaintMessageAction struct {
	ActionType          string                    `json:"action_type,omitempty"`
	JumpURL             string                    `json:"jump_url,omitempty"`
	MiniProgramJumpInfo *ComplaintMiniProgramJump `json:"mini_program_jump_info,omitempty"`
	MessageInfo         *ComplaintMessageInfo     `json:"message_info,omitempty"`
	ActionID            string                    `json:"action_id,omitempty"`
}

type ComplaintMiniProgramJump struct {
	AppID string `json:"appid,omitempty"`
	Path  string `json:"path,omitempty"`
}

type ComplaintMessageInfo struct {
	Content    string `json:"content,omitempty"`
	CustomData string `json:"custom_data,omitempty"`
}

type ComplaintInvalidInfo struct {
	ExpiredTime    string `json:"expired_time,omitempty"`
	MultiClickable bool   `json:"multi_clickable,omitempty"`
}

type ComplaintFAQItem struct {
	FAQID    string                  `json:"faq_id,omitempty"`
	FAQTitle string                  `json:"faq_title,omitempty"`
	Action   *ComplaintMessageAction `json:"action,omitempty"`
}

type ComplaintInnerButton struct {
	Text   string                  `json:"text,omitempty"`
	Action *ComplaintMessageAction `json:"action,omitempty"`
}
