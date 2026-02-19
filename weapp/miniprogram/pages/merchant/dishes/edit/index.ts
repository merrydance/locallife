import { getStableBarHeights } from '../../../../utils/responsive'
import { DishManagementService, CreateDishRequest, UpdateDishRequest } from '../../../../api/dish'
import { API_BASE } from '../../../../utils/request'
import { logger } from '../../../../utils/logger'

interface UploadFileItem {
  url: string
  status?: 'loading' | 'done' | 'failed'
  remotePath?: string
}

interface DishEditPageOptions {
  id?: string
}

interface FormInputDetail {
  value: string
}

interface CategoryOption {
  label: string
  value: string
}

Page({
  data: {
    navBarHeight: 88,
    isEdit: false,
    dishId: 0,
    loading: false,
    submitting: false,
    imageUploading: false,
    isIPhoneX: false,
    formData: {
      name: '',
      description: '',
      category_id: 0,
      price: 0, // 分
      member_price: 0, // 分
      is_online: true,
      is_available: true,
      prepare_time: 15,
      image_url: ''
    },
    displayPrice: '', // 元
    displayMemberPrice: '', // 元
    selectedCategoryName: '',
    selectedCategoryValue: '',
    categoryVisible: false,
    categoryOptions: [] as CategoryOption[],
    fileList: [] as UploadFileItem[]
  },

  onLoad(options: DishEditPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    const deviceInfo = wx.getDeviceInfo ? wx.getDeviceInfo() : wx.getSystemInfoSync()
    const model = deviceInfo?.model || ''
    const isIPhoneX = model.includes('iPhone X') || model.includes('iPhone 11') || model.includes('iPhone 12') || model.includes('iPhone 13')
    
    this.setData({ 
      navBarHeight, 
      isIPhoneX,
      isEdit: !!options.id,
      dishId: options.id ? Number(options.id) : 0
    })

    this.loadCategories()
    if (this.data.isEdit) {
      this.loadDishDetail()
    }
  },

  async loadCategories() {
    try {
      const list = await DishManagementService.getDishCategories()
      const categoryOptions = list.map((c) => ({ label: c.name, value: String(c.id) }))
      const updates: WechatMiniprogram.Page.DataOption = { categoryOptions }

      if (this.data.isEdit && this.data.formData.category_id > 0) {
        const hit = categoryOptions.find((item) => Number(item.value) === this.data.formData.category_id)
        if (hit) {
          updates.selectedCategoryValue = hit.value
          updates.selectedCategoryName = hit.label
        }
      }

      if (!this.data.isEdit && this.data.formData.category_id <= 0 && categoryOptions.length > 0) {
        updates['formData.category_id'] = Number(categoryOptions[0].value)
        updates.selectedCategoryValue = categoryOptions[0].value
        updates.selectedCategoryName = categoryOptions[0].label
      }
      this.setData(updates)
    } catch (err) {
      logger.error('Load categories failed', err)
    }
  },

  async loadDishDetail() {
    this.setData({ loading: true })
    try {
      // 这里的 API getDishDetail 获取的是详情
      const res = await DishManagementService.getDishDetail(this.data.dishId)
      this.setData({
        formData: {
          name: res.name,
          description: res.description,
          category_id: res.category_id || 0,
          price: res.price,
          member_price: res.member_price || 0,
          is_online: res.is_online,
          is_available: res.is_available,
          prepare_time: res.prepare_time || 15,
          image_url: res.image_url
        },
        displayPrice: (res.price / 100).toFixed(2),
        displayMemberPrice: res.member_price ? (res.member_price / 100).toFixed(2) : '',
        fileList: res.image_url ? [{ url: this.buildPreviewUrl(res.image_url), remotePath: res.image_url, status: 'done' }] : [],
        selectedCategoryName: res.category_name || '',
        selectedCategoryValue: res.category_id ? String(res.category_id) : ''
      })
    } catch (err) {
      logger.error('Load dish detail failed', err)
      wx.showToast({ title: '加载菜品失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  // ==================== 输入处理 ====================

  onInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) return
    const { value } = e.detail
    if (field === 'prepare_time') {
      const prepareTime = Number.parseInt(value, 10)
      this.setData({ [`formData.${field}`]: Number.isFinite(prepareTime) ? prepareTime : 0 })
      return
    }
    this.setData({ [`formData.${field}`]: value.trimStart() })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) return
    const { value } = e.detail
    this.setData({ [`formData.${field}`]: value })
  },

  onPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const val = e.detail.value.trim()
    const parsed = Number.parseFloat(val)
    this.setData({
      displayPrice: val,
      'formData.price': Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 0
    })
  },

  onMemberPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const val = e.detail.value.trim()
    const parsed = Number.parseFloat(val)
    this.setData({
      displayMemberPrice: val,
      'formData.member_price': Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 0
    })
  },

  // ==================== 图片处理 ====================

  buildPreviewUrl(path: string): string {
    if (!path) return ''
    if (path.startsWith('http://') || path.startsWith('https://') || path.startsWith('wxfile://') || path.startsWith('data:')) {
      return path
    }
    if (path.startsWith('//')) {
      return `https:${path}`
    }
    if (path.startsWith('/')) {
      return `${API_BASE}${path}`
    }
    return `${API_BASE}/${path}`
  },

  async onImageAdd(e: WechatMiniprogram.CustomEvent<{ files: Array<{ url: string }> }>) {
    const files = Array.isArray(e.detail?.files) ? e.detail.files : []
    const localPath = files[0]?.url
    if (!localPath) {
      wx.showToast({ title: '请选择有效图片', icon: 'none' })
      return
    }

    this.setData({
      imageUploading: true,
      fileList: [{ url: localPath, status: 'loading' }]
    })

    try {
      const remoteUrl = await DishManagementService.uploadDishImage(localPath)
      this.setData({
        fileList: [{ url: this.buildPreviewUrl(remoteUrl), remotePath: remoteUrl, status: 'done' }],
        'formData.image_url': remoteUrl
      })
      wx.showToast({ title: '上传成功', icon: 'success' })
    } catch (err) {
      logger.error('Upload image failed', err)
      this.setData({ fileList: [] })
      wx.showToast({ title: '上传失败', icon: 'none' })
    } finally {
      this.setData({ imageUploading: false })
    }
  },

  onImagePreview(e: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    const urls = this.data.fileList
      .map((item) => item.url)
      .filter((url) => !!url)

    if (!urls.length) return

    const index = Number(e.detail?.index || 0)
    wx.previewImage({
      current: urls[index] || urls[0],
      urls
    })
  },

  onImageRemove() {
    this.setData({
      fileList: [],
      'formData.image_url': ''
    })
  },

  // ==================== 分类选择 ====================

  showCategoryPicker() {
    if (!this.data.categoryOptions.length) {
      wx.showToast({ title: '暂无分类，请先创建分类', icon: 'none' })
      return
    }

    this.setData({ categoryVisible: true })
  },

  onCategoryConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null, label: string[] | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const labels = Array.isArray(e.detail?.label) ? e.detail.label : []
    const selectedValue = String(values[0] ?? this.data.selectedCategoryValue ?? '')
    const val = Number(selectedValue || this.data.formData.category_id)
    const label = String(labels[0] ?? this.data.selectedCategoryName ?? '')

    if (!Number.isFinite(val) || val <= 0) {
      this.setData({ categoryVisible: false })
      wx.showToast({ title: '请选择分类', icon: 'none' })
      return
    }

    this.setData({
      'formData.category_id': val,
      selectedCategoryValue: selectedValue,
      selectedCategoryName: label,
      categoryVisible: false
    })
  },

  onCategoryCancel() {
    this.setData({ categoryVisible: false })
  },

  // ==================== 提交 ====================

  async ensureCategoryForSubmit(): Promise<number> {
    if (this.data.formData.category_id > 0) {
      return this.data.formData.category_id
    }

    if (this.data.categoryOptions.length > 0) {
      const first = this.data.categoryOptions[0]
      this.setData({
        'formData.category_id': Number(first.value),
        selectedCategoryValue: first.value,
        selectedCategoryName: first.label
      })
      return Number(first.value)
    }

    const created = await DishManagementService.createDishCategory({
      name: '默认分类',
      sort_order: 99
    })
    const createdOption: CategoryOption = { label: created.name, value: String(created.id) }
    this.setData({
      categoryOptions: [createdOption],
      'formData.category_id': created.id,
      selectedCategoryValue: String(created.id),
      selectedCategoryName: created.name
    })
    return created.id
  },

  buildSubmitPayload(categoryId: number): CreateDishRequest | UpdateDishRequest {
    const name = this.data.formData.name.trim()
    const description = this.data.formData.description.trim()
    const payload: CreateDishRequest | UpdateDishRequest = {
      name,
      category_id: categoryId,
      price: this.data.formData.price,
      is_online: this.data.formData.is_online,
      is_available: this.data.formData.is_available,
      prepare_time: this.data.formData.prepare_time,
      sort_order: 0
    }

    if (description) {
      payload.description = description
    }
    if (this.data.formData.image_url) {
      payload.image_url = this.data.formData.image_url
    }
    if (this.data.formData.member_price > 0) {
      payload.member_price = this.data.formData.member_price
    }

    return payload
  },

  async onSubmit() {
    const { formData } = this.data
    if (!formData.name.trim()) {
      wx.showToast({ title: '请输入菜品名称', icon: 'none' })
      return
    }
    if (formData.price <= 0) {
      wx.showToast({ title: '请输入正确价格', icon: 'none' })
      return
    }
    if (formData.member_price > 0 && formData.member_price >= formData.price) {
      wx.showToast({ title: '会员价需小于售价', icon: 'none' })
      return
    }
    if (formData.prepare_time < 1 || formData.prepare_time > 120) {
      wx.showToast({ title: '出餐时间需在1-120分钟', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      const categoryId = await this.ensureCategoryForSubmit()
      const payload = this.buildSubmitPayload(categoryId)

      if (this.data.isEdit) {
        await DishManagementService.updateDish(this.data.dishId, payload as UpdateDishRequest)
      } else {
        await DishManagementService.createDish(payload as CreateDishRequest)
      }
      
      wx.showToast({ title: '提交成功', icon: 'success' })
      setTimeout(() => {
        const pages = getCurrentPages()
        const prevPage = pages[pages.length - 2] as { refreshAll?: () => void } | undefined
        if (prevPage?.refreshAll) {
          prevPage.refreshAll()
        }
        wx.navigateBack()
      }, 1500)
    } catch (err) {
      logger.error('Submit dish failed', err)
      wx.showToast({ title: '提交失败，请重试', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
