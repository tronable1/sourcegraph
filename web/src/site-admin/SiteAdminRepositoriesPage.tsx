import { Checkmark } from '@sourcegraph/icons/lib/Checkmark'
import { Close } from '@sourcegraph/icons/lib/Close'
import GearIcon from '@sourcegraph/icons/lib/Gear'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Subject } from 'rxjs/Subject'
import { displayRepoPath, splitPath } from '../components/Breadcrumb'
import {
    FilteredConnection,
    FilteredConnectionFilter,
    FilteredConnectionQueryArgs,
} from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import { refreshSiteFlags } from '../site/backend'
import { eventLogger } from '../tracking/eventLogger'
import { fetchAllRepositories, setRepositoryEnabled, updateMirrorRepository } from './backend'

interface RepositoryNodeProps {
    node: GQL.IRepository
    onDidUpdate?: () => void
}

interface RepositoryNodeState {
    loading: boolean
    errorDescription?: string
}

export class RepositoryNode extends React.PureComponent<RepositoryNodeProps, RepositoryNodeState> {
    public state: RepositoryNodeState = {
        loading: false,
    }

    public render(): JSX.Element | null {
        const [repoDir, repoBase] = splitPath(displayRepoPath(this.props.node.uri))

        return (
            <li
                key={this.props.node.id}
                className={`site-admin-detail-list__item site-admin-repositories-page__repo site-admin-repositories-page__repo--${
                    this.props.node.enabled ? 'enabled' : 'disabled'
                }`}
            >
                <div className="site-admin-detail-list__header site-admin-repositories-page__repo-header">
                    <Link to={`/${this.props.node.uri}/-/settings`} className="site-admin-repositories-page__repo-link">
                        {repoDir}/<strong>{repoBase}</strong>
                    </Link>
                    {this.props.node.enabled ? (
                        <small
                            data-tooltip="Access to this repository is enabled. All users can view and search it."
                            className="site-admin-repositories-page__repo-access"
                        >
                            <Checkmark className="icon-inline" />Enabled
                        </small>
                    ) : (
                        <small
                            data-tooltip="Access to this repository is disabled. Enable access to it to allow users to view and search it."
                            className="site-admin-repositories-page__repo-access"
                        >
                            <Close className="icon-inline" />Disabled
                        </small>
                    )}
                </div>
                <div className="site-admin-detail-list__actions site-admin-repositories-page__actions">
                    {
                        <Link
                            className="btn btn-secondary btn-sm site-admin-detail-list__action"
                            to={`/${this.props.node.uri}/-/settings`}
                            data-tooltip="Repository settings"
                        >
                            <GearIcon className="icon-inline" />
                        </Link>
                    }
                    {this.props.node.enabled ? (
                        <button
                            className="btn btn-secondary btn-sm site-admin-detail-list__action"
                            onClick={this.disableRepository}
                            disabled={this.state.loading}
                            data-tooltip="Disable access to the repository. Users will be unable to view and search it."
                        >
                            Disable
                        </button>
                    ) : (
                        <button
                            className="btn btn-success btn-sm site-admin-detail-list__action"
                            onClick={this.enableRepository}
                            disabled={this.state.loading}
                            data-tooltip="Enable access to the repository. Users will be able to view and search it."
                        >
                            Enable
                        </button>
                    )}
                    {this.state.errorDescription && (
                        <p className="site-admin-detail-list__error">{this.state.errorDescription}</p>
                    )}
                </div>
            </li>
        )
    }

    private enableRepository = () => this.setRepositoryEnabled(true)
    private disableRepository = () => this.setRepositoryEnabled(false)

    private setRepositoryEnabled(enabled: boolean): void {
        this.setState({
            errorDescription: undefined,
            loading: true,
        })

        setRepositoryEnabled(this.props.node.id, enabled)
            .toPromise()
            .then(() => updateMirrorRepository({ repository: this.props.node.id }).toPromise())
            .then(
                () => {
                    this.setState({ loading: false })
                    if (this.props.onDidUpdate) {
                        this.props.onDidUpdate()
                    }
                },
                err => this.setState({ loading: false, errorDescription: err.message })
            )
    }
}

interface Props extends RouteComponentProps<any> {}

class FilteredRepositoryConnection extends FilteredConnection<GQL.IRepository> {}

/**
 * A page displaying the repositories on this site.
 */
export class SiteAdminRepositoriesPage extends React.PureComponent<Props> {
    private static FILTERS: FilteredConnectionFilter[] = [
        {
            label: 'All',
            id: 'all',
            tooltip: 'Show all repositories',
            args: { enabled: true, disabled: true },
        },
        {
            label: 'Enabled',
            id: 'enabled',
            tooltip: 'Show access-enabled repositories only',
            args: { enabled: true, disabled: false },
        },
        {
            label: 'Disabled',
            id: 'disabled',
            tooltip: 'Show access-disabled repositories only',
            args: { enabled: false, disabled: true },
        },
    ]

    private repositoryUpdates = new Subject<void>()

    public componentDidMount(): void {
        eventLogger.logViewEvent('SiteAdminRepos')

        // Refresh global alert about enabling repositories when the user visits here.
        refreshSiteFlags()
            .toPromise()
            .then(null, err => console.error(err))
    }

    public componentWillUnmount(): void {
        // Remove global alert about enabling repositories when the user navigates away from here.
        refreshSiteFlags()
            .toPromise()
            .then(null, err => console.error(err))
    }

    public render(): JSX.Element | null {
        const nodeProps: Pick<RepositoryNodeProps, 'onDidUpdate'> = {
            onDidUpdate: this.onDidUpdateRepository,
        }

        return (
            <div className="site-admin-detail-list site-admin-repositories-page">
                <PageTitle title="Repositories" />
                <div className="site-admin-page__header">
                    <h2 className="site-admin-page__header-title">Repositories</h2>
                    <div className="site-admin-page__actions">
                        <Link
                            to="/site-admin/configuration"
                            className="btn btn-primary btn-sm site-admin-page__actions-btn"
                        >
                            <GearIcon className="icon-inline" /> Configure repositories
                        </Link>
                    </div>
                </div>
                <FilteredRepositoryConnection
                    className="site-admin-page__filtered-connection"
                    noun="repository"
                    pluralNoun="repositories"
                    queryConnection={this.queryRepositories}
                    nodeComponent={RepositoryNode}
                    nodeComponentProps={nodeProps}
                    updates={this.repositoryUpdates}
                    filters={SiteAdminRepositoriesPage.FILTERS}
                    history={this.props.history}
                    location={this.props.location}
                />
            </div>
        )
    }

    private queryRepositories = (args: FilteredConnectionQueryArgs) => fetchAllRepositories({ ...args })

    private onDidUpdateRepository = () => this.repositoryUpdates.next()
}
