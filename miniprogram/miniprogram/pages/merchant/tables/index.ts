/**
 * 桌台管理页面
 * 两栏布局 + 两步向导：
 * 第一步：基本信息 -> 保存创建桌台
 * 第二步：图片上传、标签管理、二维码
 */
import {
    tableManagementService,
    TableResponse,
    CreateTableRequest,
    UpdateTableRequest,
    TableImageResponse,
    TagInfo
} from '../../../api/table-device-management'
import { TagService } from '../../../api/dish'
import { logger } from '../../../utils/logger'
import { resolveImageURL } from '../../../utils/image-security'
import { API_BASE } from '../../../utils/request'

const app = getApp<IAppOption>()

// 空桌台模板
const emptyTable = (): Partial<TableResponse> => ({
    table_no: '',
    table_type: 'table',
    capacity: 0,
    description: '',
    minimum_spend: undefined,
    status: 'available'
})

Page({
    data: {
        // 侧边栏状态
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,

        // 加载状态
        loading: true,
        saving: false,

        // 桌台数据
        tables: [] as TableResponse[],
        filteredTables: [] as TableResponse[],

        // 筛选
        activeType: '',

        // 编辑状态
        selectedTable: null as Partial<TableResponse> | null,
        isAdding: false,
        currentStep: 1,

        // 最低消费（元）
        minimumSpendYuan: '',

        // 第二步数据
        tableImages: [] as TableImageResponse[],
        qrCodeUrl: '',

        // 标签管理
        availableTableTags: [] as TagInfo[],  // 可用标签列表
        selectedTagIds: [] as number[],        // 已选标签ID
        showTagManager: false,                 // 显示标签管理弹窗
        newTagName: ''                         // 新标签名称
    },

    onLoad() {
        this.initData()
    },

    onShow() {
        if (this.data.tables.length > 0) {
            this.loadTables()
        }
    },

    async initData() {
        const merchantId = app.globalData.merchantId

        if (merchantId) {
            this.setData({ merchantName: (app.globalData as any).merchantName || '' })
            await Promise.all([
                this.loadTables(),
                this.loadAvailableTableTags()
            ])
        } else {
            app.userInfoReadyCallback = async () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantName: (app.globalData as any).merchantName || '' })
                    await Promise.all([
                        this.loadTables(),
                        this.loadAvailableTableTags()
                    ])
                }
            }
        }
    },

    // ========== 数据加载 ==========
    async loadTables() {
        this.setData({ loading: true })

        try {
            const response = await tableManagementService.listTables()
            const tables = response.tables || []

            this.setData({ tables, loading: false })
            this.applyFilter()
        } catch (error) {
            logger.error('加载桌台列表失败', error, 'Tables')
            this.setData({ loading: false })
            wx.showToast({ title: '加载失败', icon: 'none' })
        }
    },

    // ========== 筛选 ==========
    onTypeFilter(e: WechatMiniprogram.TouchEvent) {
        const type = e.currentTarget.dataset.type || ''
        this.setData({ activeType: type })
        this.applyFilter()
    },

    applyFilter() {
        const { tables, activeType } = this.data
        let filtered = [...tables]
        if (activeType) {
            filtered = filtered.filter(t => t.table_type === activeType)
        }
        this.setData({ filteredTables: filtered })
    },

    // ========== 选择/添加 ==========
    async onSelectTable(e: WechatMiniprogram.TouchEvent) {
        const item = e.currentTarget.dataset.item as TableResponse
        const minSpend = item.minimum_spend ? String(item.minimum_spend / 100) : ''

        // 提取已选标签 ID
        const selectedTagIds = (item.tags || []).map((t: TagInfo) => t.id)

        this.setData({
            selectedTable: { ...item },
            isAdding: false,
            currentStep: 1,
            minimumSpendYuan: minSpend,
            tableImages: [],
            selectedTagIds,
            qrCodeUrl: ''
        })

        // 加载图片和二维码
        await this.loadTableExtras(item.id)
    },

    async loadTableExtras(tableId: number) {
        try {
            const [imagesRes, qrRes] = await Promise.all([
                tableManagementService.getTableImages(tableId).catch(() => ({ images: [] })),
                tableManagementService.getTableQRCode(tableId).catch(() => ({ qr_code_url: '' }))
            ])

            const images: TableImageResponse[] = []
            for (const img of (imagesRes.images || [])) {
                const resolvedUrl = await resolveImageURL(img.image_url || '')
                images.push({ ...img, image_url: resolvedUrl })
            }

            // 解析二维码URL为完整路径
            let qrCodeUrl = ''
            if (qrRes.qr_code_url) {
                qrCodeUrl = await resolveImageURL(qrRes.qr_code_url)
            }

            this.setData({
                tableImages: images,
                qrCodeUrl
            })
        } catch (error: any) {
            logger.error('加载桌台附加信息失败', error, 'Tables')
        }
    },

    onAddTable() {
        this.setData({
            selectedTable: emptyTable(),
            isAdding: true,
            currentStep: 1,
            minimumSpendYuan: '',
            tableImages: [],
            selectedTagIds: [],
            newTagName: '',
            qrCodeUrl: ''
        })
    },

    onCancelEdit() {
        this.setData({
            selectedTable: null,
            isAdding: false,
            currentStep: 1
        })
    },

    // ========== 表单输入 ==========
    onFieldChange(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field as string
        this.setData({ [`selectedTable.${field}`]: e.detail.value })
    },

    onNumberFieldChange(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field as string
        const value = e.detail.value ? parseInt(e.detail.value) : undefined
        this.setData({ [`selectedTable.${field}`]: value })
    },

    onMinSpendChange(e: WechatMiniprogram.Input) {
        const yuan = e.detail.value
        this.setData({ minimumSpendYuan: yuan })
        const fen = yuan ? Math.round(parseFloat(yuan) * 100) : undefined
        this.setData({ 'selectedTable.minimum_spend': fen })
    },

    onSelectType(e: WechatMiniprogram.TouchEvent) {
        const type = e.currentTarget.dataset.type
        this.setData({ 'selectedTable.table_type': type })
    },

    onSelectStatus(e: WechatMiniprogram.TouchEvent) {
        const status = e.currentTarget.dataset.status
        this.setData({ 'selectedTable.status': status })
    },

    // ========== 两步向导 ==========
    async onNextStep() {
        const { selectedTable } = this.data
        if (!selectedTable) return

        if (!selectedTable.table_no?.trim()) {
            wx.showToast({ title: '请输入桌号', icon: 'none' })
            return
        }
        if (!selectedTable.capacity || selectedTable.capacity < 1) {
            wx.showToast({ title: '请输入有效人数', icon: 'none' })
            return
        }

        this.setData({ saving: true })

        try {
            const createData: CreateTableRequest = {
                table_no: selectedTable.table_no!.trim(),
                table_type: selectedTable.table_type as 'table' | 'room',
                capacity: selectedTable.capacity,
                description: selectedTable.description?.trim() || undefined,
                minimum_spend: selectedTable.minimum_spend || undefined,
                tag_ids: this.data.selectedTagIds.length > 0 ? this.data.selectedTagIds : undefined
            }

            const newTable = await tableManagementService.createTable(createData)

            this.setData({
                saving: false,
                currentStep: 2,
                selectedTable: newTable
            })

            wx.showToast({ title: '桌台已创建', icon: 'success' })
            this.loadTables()
        } catch (error: any) {
            logger.error('创建桌台失败', error, 'Tables')
            this.setData({ saving: false })
            wx.showToast({ title: error?.userMessage || '创建失败', icon: 'none' })
        }
    },

    async onFinishAdd() {
        // 如果有选择标签，保存到已创建的桌台
        const { selectedTable, selectedTagIds } = this.data
        if (selectedTable?.id && selectedTagIds.length > 0) {
            try {
                await tableManagementService.updateTable(selectedTable.id, {
                    tag_ids: selectedTagIds
                })
            } catch (error) {
                logger.error('保存标签失败', error, 'Tables')
            }
        }

        this.setData({
            selectedTable: null,
            isAdding: false,
            currentStep: 1,
            selectedTagIds: []  // 重置标签选择
        })
        this.loadTables()
    },

    // ========== 保存（编辑模式） ==========
    async onSaveTable() {
        const { selectedTable } = this.data
        if (!selectedTable?.id) return

        if (!selectedTable.table_no?.trim()) {
            wx.showToast({ title: '请输入桌号', icon: 'none' })
            return
        }
        if (!selectedTable.capacity || selectedTable.capacity < 1) {
            wx.showToast({ title: '请输入有效人数', icon: 'none' })
            return
        }

        this.setData({ saving: true })

        try {
            const updateData: UpdateTableRequest = {
                table_no: selectedTable.table_no?.trim(),
                capacity: selectedTable.capacity,
                description: selectedTable.description?.trim() || undefined,
                minimum_spend: selectedTable.minimum_spend || undefined,
                status: selectedTable.status as 'available' | 'occupied' | 'disabled',
                tag_ids: this.data.selectedTagIds  // 添加标签ID列表
            }

            await tableManagementService.updateTable(selectedTable.id, updateData)

            this.setData({ saving: false })
            wx.showToast({ title: '保存成功', icon: 'success' })

            await this.loadTables()
        } catch (error: any) {
            logger.error('保存桌台失败', error, 'Tables')
            this.setData({ saving: false })
            wx.showToast({ title: error?.userMessage || '保存失败', icon: 'none' })
        }
    },

    // ========== 删除 ==========
    onDeleteTable() {
        const { selectedTable } = this.data
        if (!selectedTable?.id) return

        const tableNo = selectedTable.table_no || ''

        wx.showModal({
            title: '确认删除',
            content: '确定要删除桌台 ' + tableNo + ' 吗？',
            confirmColor: '#ff4d4f',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        await tableManagementService.deleteTable(selectedTable.id!)
                        wx.showToast({ title: '已删除', icon: 'success' })
                        this.setData({ selectedTable: null, isAdding: false })
                        await this.loadTables()
                    } catch (error: any) {
                        logger.error('删除失败', error, 'Tables')
                        wx.showToast({ title: error?.userMessage || '删除失败', icon: 'none' })
                    }
                }
            }
        })
    },

    // ========== 图片管理 ==========
    async onUploadImage() {
        const tableId = this.data.selectedTable?.id
        if (!tableId) return

        try {
            const res = await wx.chooseMedia({
                count: 1,
                mediaType: ['image'],
                sourceType: ['album', 'camera']
            })

            const tempPath = res.tempFiles[0].tempFilePath
            wx.showLoading({ title: '上传中...' })

            // 上传图片到服务器
            const { getToken } = require('../../../utils/auth')
            const token = getToken()
            const uploadRes = await new Promise<string>((resolve, reject) => {
                wx.uploadFile({
                    url: API_BASE + '/v1/tables/images/upload',
                    filePath: tempPath,
                    name: 'image',
                    header: { 'Authorization': 'Bearer ' + token },
                    success: (uploadResult) => {
                        // 200 OK 或 201 Created 都表示成功
                        if (uploadResult.statusCode === 200 || uploadResult.statusCode === 201) {
                            const data = JSON.parse(uploadResult.data)
                            resolve(data.image_url || data.url || '')
                        } else {
                            reject(new Error('HTTP ' + uploadResult.statusCode))
                        }
                    },
                    fail: (err) => {
                        reject(err)
                    }
                })
            })

            // 添加到桌台
            await tableManagementService.uploadTableImage(tableId, { image_url: uploadRes })

            wx.hideLoading()
            wx.showToast({ title: '上传成功', icon: 'success' })

            await this.loadTableExtras(tableId)
        } catch (error: any) {
            wx.hideLoading()
            const errMsg = error?.message || error?.errMsg || String(error)
            logger.error('上传图片失败', error, 'Tables')
            wx.showToast({ title: errMsg.substring(0, 15) || '上传失败', icon: 'none' })
        }
    },

    async onSetPrimaryImage(e: WechatMiniprogram.TouchEvent) {
        const imageId = e.currentTarget.dataset.id
        const tableId = this.data.selectedTable?.id
        if (!tableId || !imageId) return

        try {
            await tableManagementService.setPrimaryTableImage(tableId, imageId)
            wx.showToast({ title: '已设为主图', icon: 'success' })
            await this.loadTableExtras(tableId)
        } catch (error) {
            logger.error('设置主图失败', error, 'Tables')
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    },

    async onDeleteImage(e: WechatMiniprogram.TouchEvent) {
        const imageId = e.currentTarget.dataset.id
        const tableId = this.data.selectedTable?.id
        if (!tableId || !imageId) return

        wx.showModal({
            title: '确认删除',
            content: '确定要删除这张图片吗？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        await tableManagementService.deleteTableImage(tableId, imageId)
                        wx.showToast({ title: '已删除', icon: 'success' })
                        await this.loadTableExtras(tableId)
                    } catch (error) {
                        logger.error('删除图片失败', error, 'Tables')
                        wx.showToast({ title: '删除失败', icon: 'none' })
                    }
                }
            }
        })
    },

    // ========== 标签管理 ==========
    async loadAvailableTableTags() {
        try {
            const tags = await TagService.listTags('table')
            this.setData({ availableTableTags: tags })
        } catch (error) {
            logger.error('加载标签列表失败', error, 'Tables')
        }
    },

    // 切换标签选中状态
    onTagToggle(e: WechatMiniprogram.TouchEvent) {
        const tagId = Number(e.currentTarget.dataset.id)
        const { selectedTagIds } = this.data
        const index = selectedTagIds.indexOf(tagId)

        let newIds: number[]
        if (index === -1) {
            newIds = [...selectedTagIds, tagId]
        } else {
            newIds = selectedTagIds.filter(id => id !== tagId)
        }

        this.setData({ selectedTagIds: newIds })
    },

    // 打开标签管理弹窗
    onOpenTagManager() {
        this.setData({ showTagManager: true })
    },

    // 关闭标签管理弹窗
    onCloseTagManager() {
        this.setData({ showTagManager: false, newTagName: '' })
    },

    // 阻止事件冒泡
    stopPropagation() {
        // 空函数，仅用于阻止事件冒泡
    },

    // 输入新标签名
    onTagNameInput(e: WechatMiniprogram.Input) {
        this.setData({ newTagName: e.detail.value })
    },

    // 创建新标签
    async onCreateTag() {
        const { newTagName } = this.data
        if (!newTagName.trim()) {
            wx.showToast({ title: '请输入标签名称', icon: 'none' })
            return
        }

        try {
            const newTag = await TagService.createTag({ name: newTagName.trim(), type: 'table' })
            this.setData({
                availableTableTags: [...this.data.availableTableTags, newTag],
                newTagName: ''
            })
            wx.showToast({ title: '标签已创建', icon: 'success' })
        } catch (error) {
            logger.error('创建标签失败', error, 'Tables')
            wx.showToast({ title: '创建失败', icon: 'none' })
        }
    },

    // 删除标签
    async onDeleteTag(e: WechatMiniprogram.TouchEvent) {
        const tagId = e.currentTarget.dataset.id as number
        const tagName = e.currentTarget.dataset.name as string

        const res = await new Promise<WechatMiniprogram.ShowModalSuccessCallbackResult>(resolve => {
            wx.showModal({
                title: '确认删除',
                content: `确定要删除标签"${tagName}"吗？`,
                success: resolve
            })
        })

        if (!res.confirm) return

        try {
            await TagService.deleteTag(tagId)
            this.setData({
                availableTableTags: this.data.availableTableTags.filter(t => t.id !== tagId),
                selectedTagIds: this.data.selectedTagIds.filter(id => id !== tagId)
            })
            wx.showToast({ title: '标签已删除', icon: 'success' })
        } catch (error) {
            logger.error('删除标签失败', error, 'Tables')
            wx.showToast({ title: '删除失败', icon: 'none' })
        }
    },

    // ========== 二维码 ==========
    async onGenerateQRCode() {
        const tableId = this.data.selectedTable?.id
        if (!tableId) return

        try {
            wx.showLoading({ title: '生成中...' })
            const res = await tableManagementService.getTableQRCode(tableId)
            wx.hideLoading()

            // 解析二维码URL为完整路径
            let qrCodeUrl = ''
            if (res.qr_code_url) {
                qrCodeUrl = await resolveImageURL(res.qr_code_url)
            }

            this.setData({ qrCodeUrl })
            wx.showToast({ title: '二维码已生成', icon: 'success' })
        } catch (error) {
            wx.hideLoading()
            logger.error('生成二维码失败', error, 'Tables')
            wx.showToast({ title: '生成失败', icon: 'none' })
        }
    },

    onPreviewQRCode() {
        const { qrCodeUrl } = this.data
        if (!qrCodeUrl) return

        wx.previewImage({
            urls: [qrCodeUrl],
            current: qrCodeUrl
        })
    },

    onDownloadQRCode() {
        const { qrCodeUrl } = this.data
        if (!qrCodeUrl) {
            wx.showToast({ title: '无二维码', icon: 'none' })
            return
        }

        wx.showLoading({ title: '下载中...' })
        wx.downloadFile({
            url: qrCodeUrl,
            success: (res) => {
                wx.hideLoading()
                if (res.statusCode === 200 && res.tempFilePath) {
                    // 打开文件让用户选择保存位置
                    wx.openDocument({
                        filePath: res.tempFilePath,
                        showMenu: true, // 显示右上角菜单可保存
                        success: () => { },
                        fail: () => {
                            // openDocument 失败则预览
                            wx.previewImage({
                                urls: [qrCodeUrl],
                                current: qrCodeUrl
                            })
                            wx.showToast({ title: '请右键图片保存', icon: 'none', duration: 2000 })
                        }
                    })
                } else {
                    wx.showToast({ title: '下载失败', icon: 'none' })
                }
            },
            fail: () => {
                wx.hideLoading()
                // 下载失败，直接预览
                wx.previewImage({
                    urls: [qrCodeUrl],
                    current: qrCodeUrl
                })
                wx.showToast({ title: '请右键图片保存', icon: 'none', duration: 2000 })
            }
        })
    },

    // ========== 侧边栏 ==========
    onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    goBack() {
        wx.navigateBack()
    }
})
