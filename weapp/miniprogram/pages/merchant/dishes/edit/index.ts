import { getStableBarHeights } from '../../../../utils/responsive'
import { DishManagementService, CreateDishRequest } from '../../../../api/dish'
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

Page({
  data: {
    navBarHeight: 88,
    isEdit: false,
    dishId: 0,
    loading: false,
    submitting: false,
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
    categoryVisible: false,
    categoryOptions: [] as Array<{ label: string, value: number }>,
    fileList: [] as UploadFileItem[]
  },

  onLoad(options: DishEditPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    const { model } = wx.getSystemInfoSync()
    const isIPhoneX = model.includes('iPhone X') || model.includes('iPhone 11') || model.includes('iPhone 12') || model.includes('iPhone 13')
    
    this.setData({ 
      navBarHeight, 
      isIPhoneX,
      isEdit: !!options.id,
      dishId: options.id ? parseInt(options.id) : 0
    })

    this.loadCategories()
    if (this.data.isEdit) {
      this.loadDishDetail()
    }
  },

  async loadCategories() {
    try {
      const list = await DishManagementService.getDishCategories()
      const categoryOptions = list.map((c) => ({ label: c.name, value: c.id }))
      this.setData({ categoryOptions })
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
        fileList: res.image_url ? [{ url: res.image_url }] : [],
        selectedCategoryName: res.category_name || ''
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
    this.setData({ [`formData.${field}`]: value })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: string }
    if (!field) return
    const { value } = e.detail
    this.setData({ [`formData.${field}`]: value })
  },

  onPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const val = e.detail.value
    this.setData({
      displayPrice: val,
      'formData.price': Math.round(parseFloat(val) * 100) || 0
    })
  },

  onMemberPriceChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const val = e.detail.value
    this.setData({
      displayMemberPrice: val,
      'formData.member_price': val ? Math.round(parseFloat(val) * 100) : 0
    })
  },

  // ==================== 图片处理 ====================

  async onImageAdd(e: WechatMiniprogram.CustomEvent<{ files: Array<{ url: string }> }>) {
    const { files } = e.detail
    wx.showLoading({ title: '上传中...' })
    try {
      const remoteUrl = await DishManagementService.uploadDishImage(files[0].url)
      this.setData({
        fileList: [{ url: remoteUrl }],
        'formData.image_url': remoteUrl
      })
    } catch (err) {
      logger.error('Upload image failed', err)
      wx.showToast({ title: '上传失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  },

  onImageRemove() {
    this.setData({
      fileList: [],
      'formData.image_url': ''
    })
  },

  // ==================== 分类选择 ====================

  showCategoryPicker() {
    this.setData({ categoryVisible: true })
  },

  onCategoryConfirm(e: WechatMiniprogram.CustomEvent<{ value: number[], label: string[] }>) {
    const [val] = e.detail.value
    const [label] = e.detail.label
    this.setData({
      'formData.category_id': val,
      selectedCategoryName: label,
      categoryVisible: false
    })
  },

  onCategoryCancel() {
    this.setData({ categoryVisible: false })
  },

  // ==================== 提交 ====================

  async onSubmit() {
    const { formData } = this.data
    if (!formData.name) {
      wx.showToast({ title: '请输入菜品名称', icon: 'none' })
      return
    }
    if (formData.price <= 0) {
      wx.showToast({ title: '请输入正确价格', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      if (this.data.isEdit) {
        await DishManagementService.updateDish(this.data.dishId, formData)
      } else {
        await DishManagementService.createDish(formData as CreateDishRequest)
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
