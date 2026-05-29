declare namespace LocalDayjs {
  type Unit = 'millisecond' | 'second' | 'minute' | 'hour' | 'day' | 'week' | 'month' | 'quarter' | 'year' | 'date' | 'M' | 'y' | 'w' | 'd' | 'D' | 'h' | 'm' | 's' | 'ms' | 'Q'

  interface Dayjs {
    isValid(): boolean
    format(template?: string): string
    valueOf(): number
    add(value: number, unit?: Unit): Dayjs
    subtract(value: number, unit?: Unit): Dayjs
    startOf(unit: Unit): Dayjs
    endOf(unit: Unit): Dayjs
    minute(): number
    minute(value: number): Dayjs
    isAfter(value: ConfigType, unit?: Unit): boolean
    isBefore(value: ConfigType, unit?: Unit): boolean
    diff(value: ConfigType, unit?: Unit, floating?: boolean): number
  }

  type ConfigType = string | number | Date | Dayjs | null | undefined

  interface DayjsFactory {
    (value?: ConfigType): Dayjs
    isDayjs(value: unknown): value is Dayjs
    unix(value: number): Dayjs
  }
}

declare const dayjs: LocalDayjs.DayjsFactory
export = dayjs
