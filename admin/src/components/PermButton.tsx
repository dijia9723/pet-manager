import { Button, type ButtonProps } from 'antd'
import { usePermission } from '@/hooks/usePermission'

interface PermButtonProps extends ButtonProps {
  /** 所需权限点，缺省则始终渲染 */
  perm?: string
  /** 满足其中任意一个权限点即可 */
  anyOf?: string[]
}

/** 按权限点显隐的按钮 */
export function PermButton({ perm, anyOf, ...rest }: PermButtonProps) {
  const { has, hasAny } = usePermission()
  if (perm && !has(perm)) return null
  if (anyOf && anyOf.length > 0 && !hasAny(anyOf)) return null
  return <Button {...rest} />
}
