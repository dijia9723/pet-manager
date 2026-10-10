import { DictManager } from '@/components/biz/DictManager'
import { dictApi } from '@/api/dict'

/** 平台端：字典管理（左侧字典类型，抽屉内管理字典项，sys_dict/sys_dict_item） */
export default function DictPage() {
  return <DictManager api={dictApi} permPrefix="platform.dict" />
}
