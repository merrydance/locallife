/**
 * 表单输入组件
 * 统一处理输入、标签、必填标记等
 */
Component({
    properties: {
        // 标签文本
        label: {
            type: String,
            value: ''
        },
        // 输入值
        value: {
            type: String,
            value: ''
        },
        // 占位符
        placeholder: {
            type: String,
            value: '请输入'
        },
        // 输入类型：text, number, digit, idcard
        type: {
            type: String,
            value: 'text'
        },
        // 是否必填
        required: {
            type: Boolean,
            value: false
        },
        // 是否禁用
        disabled: {
            type: Boolean,
            value: false
        },
        // 最大长度
        maxlength: {
            type: Number,
            value: 140
        },
        // 字段名（用于事件传递）
        field: {
            type: String,
            value: ''
        },
        // 是否多行（使用 textarea）
        multiline: {
            type: Boolean,
            value: false
        },
        // 多行时的最小高度
        minHeight: {
            type: Number,
            value: 80
        }
    },

    methods: {
        /**
         * 输入变化
         */
        onInput(e: WechatMiniprogram.Input) {
            this.triggerEvent('input', {
                value: e.detail.value,
                field: this.properties.field
            })
        },

        /**
         * 失去焦点
         */
        onBlur(e: WechatMiniprogram.InputBlur) {
            this.triggerEvent('blur', {
                value: e.detail.value,
                field: this.properties.field
            })
        }
    }
})
