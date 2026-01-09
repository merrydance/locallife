import { DishManagementService, DishResponse, DishCategory } from '../../../../api/dish'
import { isLargeScreen } from '@/utils/responsive'
import { logger } from '../../../../utils/logger'

const app = getApp<IAppOption>()

Page({
  data: {
    dishId: 0,
    isEdit: false,
    merchantId: '',
    form: {
      name: '',
      category_id: 0,
      price: '',
      stock: '',
      description: '',
      image_url: ''
    },
    categories: [] as DishCategory[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    submitting: false
  },

  onLoad(options: any) {
    this.setData({ isLargeScreen: isLargeScreen() })
    if (options.id) {
      this.setData({ dishId: parseInt(options.id), isEdit: true })
      this.init(parseInt(options.id))
    } else {
      this.init()
    }
  },

  init(id?: number) {
    this.loadCategoriesAndDish(id)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadCategoriesAndDish(dishId?: number) {
    this.setData({ loading: true })
    try {
      const categories = await DishManagementService.getDishCategories()
      this.setData({ categories })

      if (dishId) {
        const dish = await DishManagementService.getDishDetail(dishId)
        if (dish) {
          this.setData({
            form: {
              name: dish.name,
              category_id: dish.category_id,
              price: (dish.price / 100).toFixed(2),
              stock: '0', // Adjust if you have daily_limit
              description: dish.description,
              image_url: dish.image_url
            }
          })
        }
      }
    } catch (error) {
      logger.error('Load failed', error, 'DishEdit')
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  onInputChange(e: WechatMiniprogram.CustomEvent) {
    const { field } = e.currentTarget.dataset
    this.setData({
      [`form.${field}`]: e.detail.value
    })
  },

  onCategoryChange(e: WechatMiniprogram.CustomEvent) {
    const index = Number(e.detail.value)
    if (index >= 0 && index < this.data.categories.length) {
      const selectedCategory = this.data.categories[index]
      this.setData({ 'form.category_id': selectedCategory.id })
    }
  },

  onChooseImage() {
    wx.chooseImage({
      count: 1,
      success: (res) => {
        const filePath = res.tempFilePaths[0]
        wx.showLoading({ title: '上传中...' })

        DishManagementService.uploadDishImage(filePath)
          .then((url: string) => {
            this.setData({ 'form.image_url': url })
            wx.hideLoading()
          })
          .catch((err: any) => {
            logger.error('上传图片失败', err, 'DishEdit.uploadImage')
            wx.hideLoading()
            wx.showToast({ title: '上传失败', icon: 'none' })
          })
      }
    })
  },

  async onSubmit() {
    const { form, isEdit, dishId } = this.data

    if (!form.name || !form.category_id || !form.price) {
      wx.showToast({ title: '请填写必填项', icon: 'none' })
      return
    }

    this.setData({ submitting: true })

    try {
      const payload: any = {
        name: form.name,
        category_id: form.category_id,
        price: Math.round(parseFloat(form.price) * 100),
        description: form.description,
        image_url: form.image_url
      }

      if (isEdit) {
        await DishManagementService.updateDish(dishId, payload)
      } else {
        await DishManagementService.createDish(payload)
      }

      wx.showToast({ title: isEdit ? '保存成功' : '创建成功', icon: 'success' })
      setTimeout(() => wx.navigateBack(), 1500)
    } catch (error) {
      logger.error('Submit failed', error, 'DishEdit')
      wx.showToast({ title: '保存失败', icon: 'none' })
      this.setData({ submitting: false })
    }
  }
})
