package groupux

import (
	localdb "github.com/cnxysoft/DDBOT-WSa/lsp/buntdb"
)

func LoadOverride(groupCode int64) (GroupUXOverride, error) {
	o := NewGroupUXOverride()
	err := localdb.GetJson(localdb.GroupUXConfigKey(groupCode), &o, localdb.IgnoreNotFoundOpt())
	if err != nil {
		return NewGroupUXOverride(), err
	}
	o.normalize()
	return o, nil
}

func SaveOverride(groupCode int64, override GroupUXOverride) error {
	override.normalize()
	if override.Empty() {
		_, err := localdb.Delete(localdb.GroupUXConfigKey(groupCode), localdb.IgnoreNotFoundOpt())
		return err
	}
	return localdb.SetJson(localdb.GroupUXConfigKey(groupCode), override)
}

func ResetOverride(groupCode int64) error {
	_, err := localdb.Delete(localdb.GroupUXConfigKey(groupCode), localdb.IgnoreNotFoundOpt())
	return err
}
