Component({
  properties: {
    package: {
      type: Object,
      value: {}
    }
  },
  methods: {
    onTap() {
      const pkg = this.properties.package as any
      if (pkg && pkg.id) {
        this.triggerEvent('click', { id: pkg.id })
      }
    },
    onAdd() {
      // 触发 add 事件，参数也可以放在 detail 中，虽然这里主要依赖 dataset
      // 父组件可以通过 bind:add 处理
      this.triggerEvent('add')
    }
  }
})
