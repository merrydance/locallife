interface FieldDataset {
  field?: 'name' | 'certificate_no' | 'bank_account_no' | 'bank_mobile'
}

Component({
  properties: {
    form: {
      type: Object,
      value: {}
    },
    disabled: {
      type: Boolean,
      value: false
    },
    submitting: {
      type: Boolean,
      value: false
    },
    formErrorMessage: {
      type: String,
      value: ''
    },
    submitLabel: {
      type: String,
      value: '提交资料'
    }
  },

  methods: {
    onInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const { field } = e.currentTarget.dataset as FieldDataset
      if (!field) {
        return
      }

      this.triggerEvent('change', {
        field,
        value: e.detail.value
      })
    },

    onInputId(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const { field } = e.currentTarget.dataset as FieldDataset
      if (!field) {
        return
      }

      this.triggerEvent('change', {
        field,
        value: String(e.detail.value || '').toUpperCase()
      })
    },

    onSubmit() {
      if (this.properties.disabled || this.properties.submitting) {
        return
      }
      this.triggerEvent('submit')
    }
  }
})
