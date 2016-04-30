// @flow

import BlobStore from "sourcegraph/blob/BlobStore";
import DefStore from "sourcegraph/def/DefStore";
import RepoStore from "sourcegraph/repo/RepoStore";
import TreeStore from "sourcegraph/tree/TreeStore";
import SearchStore from "sourcegraph/search/SearchStore";
import DashboardStore from "sourcegraph/dashboard/DashboardStore";
import BuildStore from "sourcegraph/build/BuildStore";
import UserStore from "sourcegraph/user/UserStore";

const allStores = {
	BlobStore,
	DefStore,
	RepoStore,
	TreeStore,
	SearchStore,
	DashboardStore,
	BuildStore,
	UserStore,
};

// forEach calls f(store, name) for all stores.
export function forEach(f: (store: Object, name: string) => void): void {
	Object.keys(allStores).forEach((key) => {
		f(allStores[key], key);
	});
}

// reset resets all stores with the provided data. If null is provided,
// then the stories are cleared.
export function reset(data: Object): void {
	forEach((store, name) => {
		store.reset(data ? data[name] : null);
	});
}
