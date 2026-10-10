import { ConfigManager } from '@/components/biz/ConfigManager'
import { configApi } from '@/api/config'

/** 平台端：参数配置（不改代码即可调整的系统级参数，sys_config） */
export default function ConfigPage() {
  return <ConfigManager api={configApi} permPrefix="platform.param" />
}
