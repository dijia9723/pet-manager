import { DictManager } from '@/components/biz/DictManager'
import { tenantDictApi } from '@/api/dict'

/** 租户端：字典管理（本店业务枚举，t_dict/t_dict_item，与平台字典互不相通） */
export default function TenantDictPage() {
  return <DictManager api={tenantDictApi} permPrefix="tenant.dict" />
}
