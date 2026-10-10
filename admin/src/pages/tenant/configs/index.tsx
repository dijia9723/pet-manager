import { ConfigManager } from '@/components/biz/ConfigManager'
import { tenantConfigApi } from '@/api/config'

/** 租户端：参数配置（本店自有参数，t_config，与平台参数互不相通） */
export default function TenantConfigPage() {
  return <ConfigManager api={tenantConfigApi} permPrefix="tenant.param" />
}
