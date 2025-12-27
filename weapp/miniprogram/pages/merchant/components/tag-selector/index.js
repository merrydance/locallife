"use strict";
/**
 * 标签选择器组件
 * 可复用于菜品和套餐的属性标签选择
 */
Component({
    properties: {
        // 可选的标签列表
        tags: {
            type: Array,
            value: []
        },
        // 已选中的标签 ID 列表
        selectedIds: {
            type: Array,
            value: []
        },
        // 最大可选数量（0表示无限制）
        maxCount: {
            type: Number,
            value: 10
        },
        // 是否显示管理入口
        showManage: {
            type: Boolean,
            value: false
        },
        // 标签类型（用于管理弹窗）
        tagType: {
            type: String,
            value: 'dish'
        },
        // 空状态提示文案
        emptyText: {
            type: String,
            value: '暂无可用标签'
        }
    },
    data: {
    // 内部状态
    },
    methods: {
        /**
         * 切换标签选中状态
         */
        onTagToggle(e) {
            const tagId = e.currentTarget.dataset.id;
            const { selectedIds, maxCount } = this.properties;
            let newIds;
            if (selectedIds.includes(tagId)) {
                // 已选中，移除
                newIds = selectedIds.filter((id) => id !== tagId);
            }
            else {
                // 未选中，检查数量限制
                if (maxCount > 0 && selectedIds.length >= maxCount) {
                    wx.showToast({ title: `最多选择${maxCount}个标签`, icon: 'none' });
                    return;
                }
                newIds = [...selectedIds, tagId];
            }
            // 触发变更事件
            this.triggerEvent('change', { ids: newIds });
        },
        /**
         * 打开标签管理
         */
        onOpenManage() {
            this.triggerEvent('manage', { type: this.properties.tagType });
        },
        /**
         * 检查标签是否选中
         */
        isSelected(tagId) {
            return this.properties.selectedIds.includes(tagId);
        }
    }
});
