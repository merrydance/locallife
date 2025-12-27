/**
 * 开关组件
 * 可复用于各种布尔值切换场景
 */
Component({
    properties: {
        // 开关状态
        value: {
            type: Boolean,
            value: false
        },
        // 标签文本
        label: {
            type: String,
            value: ''
        },
        // 是否禁用
        disabled: {
            type: Boolean,
            value: false
        },
        // 字段名（用于事件传递）
        field: {
            type: String,
            value: ''
        }
    },

    methods: {
        /**
         * 切换开关状态
         */
        onToggle() {
            if (this.properties.disabled) return

            const newValue = !this.properties.value
            this.triggerEvent('change', {
                value: newValue,
                field: this.properties.field
            })
        }
    }
})
