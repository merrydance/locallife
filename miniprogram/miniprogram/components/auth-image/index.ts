
import { getToken } from '../../utils/auth'
import { API_BASE } from '../../utils/request'
import { resolveImageURL } from '../../utils/image-security'

Component({
    externalClasses: ['class', 'custom-class'],

    properties: {
        src: {
            type: String,
            observer(newVal) {
                if (newVal) {
                    this.loadImage(newVal)
                }
            }
        },
        mode: {
            type: String,
            value: 'scaleToFill'
        },
        lazyLoad: {
            type: Boolean,
            value: false
        },
        showMenuByLongpress: {
            type: Boolean,
            value: false
        },
        className: {
            type: String,
            value: ''
        },
        customStyle: {
            type: String,
            value: ''
        }
    },

    data: {
        localPath: '',
        loading: false,
        error: false
    },

    methods: {
        async loadImage(url: string) {
            // 如果是本地路径或base64，直接显示
            if (!url || url.startsWith('wxfile://') || url.startsWith('http://tmp/') || url.startsWith('data:image')) {
                this.setData({ localPath: url, loading: false, error: false })
                return
            }

            this.setData({ loading: true, error: false })

            try {
                // Determine if we need to sign it
                const resolvedUrl = await resolveImageURL(url)

                this.setData({
                    localPath: resolvedUrl, // Use localPath to store the displayable URL
                    loading: false
                })
                this.triggerEvent('load', { path: resolvedUrl })

            } catch (e) {
                console.error('Failed to resolve image URL', e)
                this.setData({ loading: false, error: true })
                this.triggerEvent('error', e)
            }
        },

        retry() {
            if (this.properties.src) {
                this.loadImage(this.properties.src)
            }
        },

        onError(e: any) {
            this.triggerEvent('error', e)
        },

        onLoad(e: any) {
            this.triggerEvent('load', e)
        }
    }
})
