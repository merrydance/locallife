export {}

interface FormInputDetail {
  value: string
}

interface CustomizationOptionDraft {
  key: string
  name: string
  extraPriceYuan: string
}

function buildDraftKey(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

function createEmptyOptionDraft(): CustomizationOptionDraft {
  return {
    key: buildDraftKey('option'),
    name: '',
    extraPriceYuan: ''
  }
}

function cloneOptions(options: CustomizationOptionDraft[]): CustomizationOptionDraft[] {
  return (Array.isArray(options) ? options : []).map((option) => ({
    key: option.key,
    name: option.name,
    extraPriceYuan: option.extraPriceYuan
  }))
}

function normalizeOptions(options: CustomizationOptionDraft[]): CustomizationOptionDraft[] {
  const nextOptions = cloneOptions(options)
  return nextOptions.length ? nextOptions : [createEmptyOptionDraft()]
}

Component({
  options: {
    styleIsolation: 'apply-shared'
  },

  properties: {
    value: {
      type: Array,
      value: [],
      observer(value: CustomizationOptionDraft[]) {
        this.setData({ internalOptions: normalizeOptions(value || []) })
      }
    }
  },

  data: {
    internalOptions: normalizeOptions([])
  },

  methods: {
    emitChange(options: CustomizationOptionDraft[]) {
      const nextOptions = cloneOptions(options)
      this.setData({ internalOptions: normalizeOptions(nextOptions) })
      this.triggerEvent('change', { value: nextOptions })
    },

    onOptionNameChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
      const { index } = e.currentTarget.dataset as { index?: number }
      if (typeof index !== 'number') {
        return
      }

      const nextOptions = cloneOptions(this.data.internalOptions)
      nextOptions[index] = {
        ...nextOptions[index],
        name: (e.detail.value || '').replace(/^\s+/, '')
      }
      this.emitChange(nextOptions)
    },

    onOptionPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
      const { index } = e.currentTarget.dataset as { index?: number }
      if (typeof index !== 'number') {
        return
      }

      const nextOptions = cloneOptions(this.data.internalOptions)
      nextOptions[index] = {
        ...nextOptions[index],
        extraPriceYuan: (e.detail.value || '').trim()
      }
      this.emitChange(nextOptions)
    },

    onAddAfter(e: WechatMiniprogram.TouchEvent) {
      const { index } = e.currentTarget.dataset as { index?: number }
      if (typeof index !== 'number') {
        return
      }

      const nextOptions = cloneOptions(this.data.internalOptions)
      nextOptions.splice(index + 1, 0, createEmptyOptionDraft())
      this.emitChange(nextOptions)
    },

    onRemoveAt(e: WechatMiniprogram.TouchEvent) {
      const { index } = e.currentTarget.dataset as { index?: number }
      if (typeof index !== 'number') {
        return
      }

      const nextOptions = cloneOptions(this.data.internalOptions)
      if (nextOptions.length <= 1) {
        this.setData({ internalOptions: [createEmptyOptionDraft()] })
        this.triggerEvent('change', { value: [] })
        return
      }

      nextOptions.splice(index, 1)
      this.emitChange(nextOptions)
    }
  }
})