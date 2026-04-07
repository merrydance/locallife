export {}

interface FormInputDetail {
  value: string
}

interface CustomizationOptionDraft {
  key: string
  name: string
  extraPriceYuan: string
}

interface CustomizationGroupDraft {
  key: string
  name: string
  is_required: boolean
  options: CustomizationOptionDraft[]
}

const MAX_CUSTOMIZATION_GROUPS = 20

function buildDraftKey(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

function createEmptyGroupDraft(): CustomizationGroupDraft {
  return {
    key: buildDraftKey('group'),
    name: '',
    is_required: true,
    options: []
  }
}

function cloneGroups(groups: CustomizationGroupDraft[]): CustomizationGroupDraft[] {
  return (Array.isArray(groups) ? groups : []).map((group) => ({
    key: group.key,
    name: group.name,
    is_required: !!group.is_required,
    options: (Array.isArray(group.options) ? group.options : []).map((option) => ({
      key: option.key,
      name: option.name,
      extraPriceYuan: option.extraPriceYuan
    }))
  }))
}

Component({
  options: {
    styleIsolation: 'apply-shared'
  },

  properties: {
    value: {
      type: Array,
      value: [],
      observer(value: CustomizationGroupDraft[]) {
        this.setData({ internalGroups: cloneGroups(value || []) })
      }
    }
  },

  data: {
    internalGroups: [] as CustomizationGroupDraft[]
  },

  methods: {
    emitChange(groups: CustomizationGroupDraft[]) {
      const nextGroups = cloneGroups(groups)
      this.setData({ internalGroups: nextGroups })
      this.triggerEvent('change', { value: nextGroups })
    },

    openAddGroupDialog() {
      this.onAddGroup()
    },

    onAddGroup() {
      if (this.data.internalGroups.length >= MAX_CUSTOMIZATION_GROUPS) {
        wx.showToast({ title: `最多添加${MAX_CUSTOMIZATION_GROUPS}组规格`, icon: 'none' })
        return
      }

      const nextGroups = cloneGroups(this.data.internalGroups)
      nextGroups.unshift(createEmptyGroupDraft())
      this.emitChange(nextGroups)
    },

    onGroupNameChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
      const { groupIndex } = e.currentTarget.dataset as { groupIndex?: number }
      if (typeof groupIndex !== 'number') {
        return
      }

      const nextGroups = cloneGroups(this.data.internalGroups)
      nextGroups[groupIndex] = {
        ...nextGroups[groupIndex],
        name: (e.detail.value || '').replace(/^\s+/, '')
      }
      this.emitChange(nextGroups)
    },

    onRemoveGroup(e: WechatMiniprogram.TouchEvent) {
      const { groupIndex } = e.currentTarget.dataset as { groupIndex?: number }
      if (typeof groupIndex !== 'number') {
        return
      }

      const targetGroup = this.data.internalGroups[groupIndex]
      const groupName = targetGroup?.name?.trim()

      wx.showModal({
        title: '删除规格组',
        content: groupName
          ? `删除“${groupName}”后，组内规格项会一起移除。`
          : '删除后，组内规格项会一起移除。',
        success: (res) => {
          if (!res.confirm) {
            return
          }

          const nextGroups = cloneGroups(this.data.internalGroups)
          nextGroups.splice(groupIndex, 1)
          this.emitChange(nextGroups)
        }
      })
    },

    onOptionsChange(e: WechatMiniprogram.CustomEvent<{ value?: CustomizationOptionDraft[] }>) {
      const { groupIndex } = e.currentTarget.dataset as { groupIndex?: number }
      if (typeof groupIndex !== 'number') {
        return
      }

      const nextGroups = cloneGroups(this.data.internalGroups)
      nextGroups[groupIndex] = {
        ...nextGroups[groupIndex],
        options: Array.isArray(e.detail?.value) ? e.detail.value : []
      }

      this.emitChange(nextGroups)
    }
  }
})