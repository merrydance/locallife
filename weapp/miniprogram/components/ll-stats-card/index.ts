import { responsiveBehavior } from '../../utils/responsive';

Component({
    behaviors: [responsiveBehavior],

    properties: {
        label: {
            type: String,
            value: ''
        },
        value: {
            type: String,
            optionalTypes: [Number],
            value: '0'
        },
        unit: {
            type: String,
            value: ''
        },
        trend: {
            type: Number, // Positive for up, negative for down
            value: 0
        },
        icon: {
            type: String, // TDesign icon name
            value: ''
        },
        variant: {
            type: String, // 'primary' | 'secondary'
            value: 'primary'
        }
    },

    data: {
        // Local state if needed
    },

    methods: {
        onTap() {
            this.triggerEvent('tap');
        }
    }
});
