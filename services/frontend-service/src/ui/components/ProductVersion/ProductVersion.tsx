/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import './ProductVersion.scss';
import * as React from 'react';
import { useState } from 'react';
import {
    addAction,
    refreshTags,
    showSnackbarError,
    TagResponse,
    TagsWithFilter,
    useEnvironmentGroups,
    useEnvironments,
    useFrontendConfig,
    useTags,
} from '../../utils/store';
import { DisplayManifestLink, DisplaySourceLink } from '../../utils/Links';
import { Spinner } from '../Spinner/Spinner';
import {
    EnvironmentGroup,
    GetProductSummaryResponse,
    ProductSummary,
    ReleaseTrainRequest_TargetType,
} from '../../../api/api';
import { useSearchParams } from 'react-router-dom';
import { Button } from '../button';
import { useApi } from '../../utils/GrpcApi';
import { EnvSelectionDialogTrain } from '../SelectionDialog/SelectionDialogs';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';

export type TableProps = {
    productSummary: ProductSummary[];
    teams: string[];
};

export const TableFiltered: React.FC<TableProps> = (props) => {
    const { configs } = useFrontendConfig((c) => c);
    const versionToDisplay = (app: ProductSummary): string => {
        if (app.displayVersion !== '') {
            return app.displayVersion;
        }
        if (app.commitId !== '') {
            return app.commitId;
        }
        if (configs.revisionsEnabled) {
            return app.version + '.' + app.revision;
        }
        return app.version;
    };
    const displayTeams = props.teams;
    if (displayTeams.includes('<No Team>')) {
        displayTeams.filter((team) => team !== '<No Team>');
        displayTeams.push('');
    }
    return (
        <table className="table">
            <tbody>
                <tr className="table_title">
                    <th>App Name</th>
                    <th>Version</th>
                    <th>Environment</th>
                    <th>Team</th>
                    <th>ManifestRepoLink</th>
                    <th>SourceRepoLink</th>
                </tr>
                {props.productSummary
                    .filter((row) => props.teams.length <= 0 || displayTeams.includes(row.team))
                    .map((sum) => (
                        <tr key={sum.app + sum.environment} className="table_data">
                            <td>{sum.app}</td>
                            <td>{versionToDisplay(sum)}</td>
                            <td>{sum.environment}</td>
                            <td>{sum.team}</td>
                            <td>
                                <DisplayManifestLink
                                    app={sum.app}
                                    version={{ version: Number(sum.version), revision: Number(sum.revision) }}
                                    displayString="Manifest Link"
                                />
                            </td>
                            <td>
                                <DisplaySourceLink commitId={sum.commitId} displayString={'Source Link'} />
                            </td>
                        </tr>
                    ))}
            </tbody>
        </table>
    );
};

// splits up a string like "dev:dev-de" into ["dev", "dev-de"]
const splitCombinedGroupName = (envName: string): string[] => {
    const splitter = envName.split('/');
    if (splitter.length === 1) {
        return ['', splitter[0]];
    }
    return [splitter[1], ''];
};

const useEnvironmentGroupCombinations = (envGroupResponse: EnvironmentGroup[]): string[] => {
    const envList: string[] = [];
    for (let i = 0; i < envGroupResponse.length; i++) {
        for (let j = 0; j < envGroupResponse[i].environments.length; j++) {
            envList.push(envGroupResponse[i].environmentGroupName + '/' + envGroupResponse[i].environments[j].name);
        }
    }
    return envList;
};

type ProductSummaries = {
    summaries: ProductSummary[];
    error: string | null;
};

type ErrorMessageProps = {
    message: string;
};

const ErrorMessageContainer: React.FC<ErrorMessageProps> = (props) => (
    <div className={'warning-container'}>
        <div className={'warning-message'}>{props.message}</div>
    </div>
);

export const ProductVersion: React.FC = () => {
    const { authHeader } = useAzureAuthSub((auth) => auth);
    React.useEffect(() => {
        refreshTags(authHeader);
    }, [authHeader]);

    const envGroupResponse = useEnvironmentGroups();
    const envList = useEnvironmentGroupCombinations(envGroupResponse);
    const [searchParams, setSearchParams] = useSearchParams();
    const [environment, setEnvironment] = React.useState(searchParams.get('env') || envList[0]);

    const [summaryLoading, setSummaryLoading] = useState(false);
    const [productSummaries, setProductSummaries] = useState<ProductSummaries>({ summaries: [], error: null });

    const teams = (searchParams.get('teams') || '').split(',').filter((val) => val !== '');
    const [selectedTag, setSelectedTag] = React.useState(() => searchParams.get('tag') || '');
    const [tagSearch, setTagSearch] = React.useState(() => searchParams.get('tagFilter') || '');
    const [dateSearch, setDateSearch] = React.useState(() => searchParams.get('dateFilter') || '');
    const envsList = useEnvironments();
    const { tagsResponse, filteredTagData }: TagsWithFilter = useTags();
    // Persist the filters in the url so they survive a page refresh. Use replace so that typing
    // does not create a new browser-history entry per keystroke.
    const setFilterParam = React.useCallback(
        (key: string, value: string) => {
            if (value === '') {
                searchParams.delete(key);
            } else {
                searchParams.set(key, value);
            }
            setSearchParams(searchParams, { replace: true });
        },
        [searchParams, setSearchParams]
    );
    const onChangeTagSearch = React.useCallback(
        (e: React.ChangeEvent<HTMLInputElement>) => {
            setTagSearch(e.target.value);
            setFilterParam('tagFilter', e.target.value);
        },
        [setFilterParam]
    );
    const onChangeDateSearch = React.useCallback(
        (e: React.ChangeEvent<HTMLInputElement>) => {
            setDateSearch(e.target.value);
            setFilterParam('dateFilter', e.target.value);
        },
        [setFilterParam]
    );
    const searchedTagData = React.useMemo(() => {
        const tagNeedle = tagSearch.trim().toLowerCase();
        const dateNeedle = dateSearch.trim().toLowerCase();
        return tagsResponse.response.tagData.filter((tag) => {
            const matchesTag =
                tagNeedle === '' ||
                tag.tag.toLowerCase().includes(tagNeedle) ||
                tag.commitId.toLowerCase().includes(tagNeedle);
            const matchesDate =
                dateNeedle === '' ||
                (!!tag.commitDate && tag.commitDate.toISOString().toLowerCase().includes(dateNeedle));
            return matchesTag && matchesDate;
        });
    }, [tagSearch, dateSearch, tagsResponse.response.tagData]);
    // Always keep the currently selected tag in the dropdown, even when it does not match the
    // active filter. Otherwise the browser silently displays the first filtered option while the
    // selection state still points at the (now hidden) tag; re-picking that displayed option then
    // fires no change event, so the results never update.
    const dropdownTagData = React.useMemo(() => {
        if (selectedTag === '' || searchedTagData.some((tag) => tag.commitId === selectedTag)) {
            return searchedTagData;
        }
        const selected = tagsResponse.response.tagData.find((tag) => tag.commitId === selectedTag);
        return selected ? [selected, ...searchedTagData] : searchedTagData;
    }, [searchedTagData, selectedTag, tagsResponse.response.tagData]);
    const onChangeTag = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            setSelectedTag(e.target.value);
            searchParams.set('tag', e.target.value);
            setSearchParams(searchParams);
        },
        [searchParams, setSearchParams]
    );

    // When no tag is selected yet (e.g. navigated in without a tag in the url), default to the
    // latest valid tag (most recent commit date) and persist it to the url.
    React.useEffect(() => {
        if (selectedTag !== '') {
            return;
        }
        if (filteredTagData.length === 0) {
            return;
        }
        const latest = filteredTagData.reduce((a, b) => {
            if (!b.commitDate) {
                return a;
            }
            if (!a.commitDate) {
                return b;
            }
            return b.commitDate > a.commitDate ? b : a;
        });
        setSelectedTag(latest.commitId);
        searchParams.set('tag', latest.commitId);
        setSearchParams(searchParams);
    }, [selectedTag, filteredTagData, searchParams, setSearchParams]);

    // Fetch the product summary for the selected tag. This is driven by the selection (and
    // environment), not the raw url params, so changing the tag/date filters does not refetch.
    React.useEffect(() => {
        if (selectedTag === '') {
            return;
        }
        const env = splitCombinedGroupName(environment);
        useApi
            .productSummaryService()
            .GetProductSummary(
                { manifestRepoCommitHash: selectedTag, environment: env[0], environmentGroup: env[1] },
                authHeader
            )
            .then((result: GetProductSummaryResponse) => {
                setProductSummaries({ summaries: result.productSummary, error: null });
            })
            .catch((e) => {
                setProductSummaries({ summaries: [], error: e.message });
            });
        setSummaryLoading(false);
    }, [authHeader, environment, selectedTag]);

    const changeEnv = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            searchParams.set('env', e.target.value);
            setEnvironment(e.target.value);
            setSearchParams(searchParams);
        },
        [setSearchParams, searchParams]
    );
    const [showReleaseTrainEnvs, setShowReleaseTrainEnvs] = React.useState(false);
    const handleClose = React.useCallback(() => {
        setShowReleaseTrainEnvs(false);
    }, []);
    const openDialog = React.useCallback(() => {
        setShowReleaseTrainEnvs(true);
    }, []);
    const confirmReleaseTrainFunction = React.useCallback(
        (selectedEnvs: string[]) => {
            if (teams.length < 1) {
                selectedEnvs.forEach((env) => {
                    addAction({
                        action: {
                            $case: 'releaseTrain',
                            releaseTrain: {
                                target: env,
                                commitHash: selectedTag,
                                team: '',
                                ciLink: '',
                                targetType: ReleaseTrainRequest_TargetType.ENVIRONMENT,
                                gitTag: '',
                            },
                        },
                    });
                });
                setShowReleaseTrainEnvs(false);
                return;
            }
            if (teams.length > 1) {
                showSnackbarError('Can only run one release train action at a time, should only select one team');
                return;
            }
            selectedEnvs.forEach((env) => {
                addAction({
                    action: {
                        $case: 'releaseTrain',
                        releaseTrain: {
                            target: env,
                            commitHash: selectedTag,
                            team: teams[0],
                            ciLink: '',
                            targetType: ReleaseTrainRequest_TargetType.UNKNOWN,
                            gitTag: '',
                        },
                    },
                });
            });
            return;
        },
        [selectedTag, teams]
    );

    if (tagsResponse.tagsReady === TagResponse.LOADING) {
        return <Spinner message="Loading Git Tags" />;
    } else if (tagsResponse.tagsReady === TagResponse.ERROR) {
        return <ErrorMessageContainer message={'Loading Git Tags failed, please try again in a moment.'} />;
    } else if (tagsResponse.response.tagData.length === 0) {
        return <ErrorMessageContainer message={'There are no git tags in your manifest repository.'} />;
    } else if (filteredTagData.length === 0) {
        return (
            <ErrorMessageContainer
                message={
                    'There are no valid git tags in your manifest repository. ' +
                    'Git tags must be created via a call to kuberpults REST api. '
                }
            />
        );
    }
    if (summaryLoading) {
        return <Spinner message="Loading Production Version" />;
    }
    if (productSummaries.error) {
        return <ErrorMessageContainer message={productSummaries.error} />;
    }

    const groupName = splitCombinedGroupName(environment)[0];
    const envsFiltered = envsList.filter((env) => groupName === env.config?.upstream?.environment);
    const selectedTagData = tagsResponse.response.tagData.find((tag) => tag.commitId === selectedTag);

    const dialog = (
        <EnvSelectionDialogTrain
            environments={envsFiltered.map((env) => env.name)}
            open={showReleaseTrainEnvs}
            onCancel={handleClose}
            onSubmit={confirmReleaseTrainFunction}
            multiSelect={false}
        />
    );
    return (
        <div className="product_version">
            <h1 className="environment_name">{'Product Version Page'}</h1>
            {dialog}
            {tagsResponse.response.tagData.length > 0 ? (
                <div className="tag_selection_section">
                    <fieldset className="tag_filters">
                        <legend className="section_label">1. Filter tags</legend>
                        <input
                            type="text"
                            className="tag_search"
                            data-testid="tag_search"
                            placeholder="Filter by commit id or tag…"
                            value={tagSearch}
                            onChange={onChangeTagSearch}
                        />
                        <input
                            type="text"
                            className="tag_search"
                            data-testid="date_search"
                            placeholder="Filter by date…"
                            value={dateSearch}
                            onChange={onChangeDateSearch}
                        />
                    </fieldset>
                    <fieldset className="tag_selection">
                        <legend className="section_label">2. Select a tag and source environment</legend>
                        <select
                            onChange={onChangeTag}
                            className="drop_down"
                            data-testid="drop_down"
                            value={selectedTag}>
                            <option value="default" disabled>
                                Select a Tag
                            </option>
                            {dropdownTagData.map((tag) => (
                                <option value={tag.commitId} key={tag.tag} disabled={!tag.commitDate}>
                                    {tag.commitId.substring(0, 12)}
                                    {' @ '}
                                    {tag.tag.replace('refs/tags/', '')}
                                    {' @ '}
                                    {tag.commitDate ? String(tag.commitDate.toISOString()) : '(timestamp missing)'}
                                </option>
                            ))}
                        </select>
                        <select className="env_drop_down" onChange={changeEnv} value={environment}>
                            <option value="default" disabled>
                                Select a source Environment
                            </option>
                            {envList.map((env) => (
                                <option value={env} key={env}>
                                    {env}
                                </option>
                            ))}
                        </select>
                    </fieldset>
                </div>
            ) : (
                <div />
            )}
            <div>
                <div className="table_padding">
                    <div className="results_header">
                        <div className="selected_tag_banner" data-testid="selected_tag">
                            {selectedTagData ? (
                                <>
                                    Showing product version for tag{' '}
                                    <span className="selected_tag_name">
                                        {selectedTagData.tag.replace('refs/tags/', '')}
                                    </span>{' '}
                                    <span className="selected_tag_details">
                                        {'(commit '}
                                        {selectedTagData.commitId.substring(0, 12)}
                                        {selectedTagData.commitDate
                                            ? ' @ ' + String(selectedTagData.commitDate.toISOString())
                                            : ''}
                                        {')'}
                                    </span>
                                </>
                            ) : (
                                'No tag selected'
                            )}
                        </div>
                        <Button
                            label={'Run Release Train'}
                            className="release_train_button"
                            onClick={openDialog}
                            highlightEffect={false}
                        />
                    </div>
                    <TableFiltered productSummary={productSummaries.summaries} teams={teams} />
                </div>
            </div>
        </div>
    );
};
