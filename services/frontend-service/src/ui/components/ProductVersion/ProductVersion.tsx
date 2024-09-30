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
import {
    refreshTags,
    useTags,
    useEnvironmentGroups,
    useEnvironments,
    addAction,
    showSnackbarError,
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
import { useState } from 'react';
import { useApi } from '../../utils/GrpcApi';
import { EnvSelectionDialog } from '../SelectionDialog/SelectionDialogs';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';

export type TableProps = {
    productSummary: ProductSummary[];
    teams: string[];
};

export const TableFiltered: React.FC<TableProps> = (props) => {
    const versionToDisplay = (app: ProductSummary): string => {
        if (app.displayVersion !== '') {
            return app.displayVersion;
        }
        if (app.commitId !== '') {
            return app.commitId;
        }
        return app.version;
    };
    const displayTeams = props.teams;
    if (displayTeams.includes('<No Team>')) {
        displayTeams.filter((team, index) => team !== '<No Team>');
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
                    .filter((row, index) => props.teams.length <= 0 || displayTeams.includes(row.team))
                    .map((sum) => (
                        <tr key={sum.app + sum.environment} className="table_data">
                            <td>{sum.app}</td>
                            <td>{versionToDisplay(sum)}</td>
                            <td>{sum.environment}</td>
                            <td>{sum.team}</td>
                            <td>
                                <DisplayManifestLink
                                    app={sum.app}
                                    version={Number(sum.version)}
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
        envList.push(envGroupResponse[i].environmentGroupName);
        for (let j = 0; j < envGroupResponse[i].environments.length; j++) {
            envList.push(envGroupResponse[i].environmentGroupName + '/' + envGroupResponse[i].environments[j].name);
        }
    }
    return envList;
};

export const ProductVersion: React.FC = () => {
    React.useEffect(() => {
        refreshTags();
    }, []);
    const envGroupResponse = useEnvironmentGroups();
    const envList = useEnvironmentGroupCombinations(envGroupResponse);
    const [searchParams, setSearchParams] = useSearchParams();
    const [environment, setEnvironment] = React.useState(searchParams.get('env') || envList[0]);
    const [showSummary, setShowSummary] = useState(false);
    const [summaryLoading, setSummaryLoading] = useState(false);
    const [productSummaries, setProductSummaries] = useState(Array<ProductSummary>());
    const teams = (searchParams.get('teams') || '').split(',').filter((val) => val !== '');
    const [selectedTag, setSelectedTag] = React.useState('');
    const envsList = useEnvironments();
    const tagsResponse = useTags();
    const { authHeader } = useAzureAuthSub((auth) => auth);

    const onChangeTag = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            setSelectedTag(e.target.value);
            searchParams.set('tag', e.target.value);
            setSearchParams(searchParams);
        },
        [searchParams, setSearchParams]
    );
    React.useEffect(() => {
        let tag = searchParams.get('tag');

        let ts = undefined;
        if (tag === null) {
            // eslint-disable-next-line no-console
            console.log('tag: ' + tag);
            if (tagsResponse.response.tagData.length === 0) return;
            tag = tagsResponse.response.tagData[0].commitId;

            if (tag === null) return;
            setSelectedTag(tag);

            searchParams.set('tag', tag);
            setSearchParams(searchParams);
        }
        ts = tagsResponse.response.tagData.find((currentTag) => currentTag.commitId === tag)?.timestamp;
        // eslint-disable-next-line no-console
        console.log(ts);
        // eslint-disable-next-line no-console
        console.log(tagsResponse.response.tagData);

        const env = splitCombinedGroupName(environment);
        setShowSummary(true);
        setSummaryLoading(true);

        useApi
            .gitService()
            .GetProductSummary(
                {
                    manifestRepoCommitHash: tag,
                    environment: env[0],
                    environmentGroup: env[1],
                    timestamp: ts,
                },
                authHeader
            )
            .then((result: GetProductSummaryResponse) => {
                setProductSummaries(result.productSummary);
            })
            .catch((e) => {
                showSnackbarError(e.message);
            });
        setSummaryLoading(false);
    }, [tagsResponse, envGroupResponse, environment, searchParams, setSearchParams, authHeader]);

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
                                targetType: ReleaseTrainRequest_TargetType.UNKNOWN,
                            },
                        },
                    });
                });
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
                        },
                    },
                });
            });
            return;
        },
        [selectedTag, teams]
    );

    if (!tagsResponse.tagsReady) {
        return <Spinner message="Loading Git Tags" />;
    }
    if (summaryLoading) {
        return <Spinner message="Loading Production Version" />;
    }

    const dialog = (
        <EnvSelectionDialog
            environments={envsList
                .filter((env, index) => splitCombinedGroupName(environment)[0] === env.config?.upstream?.environment)
                .map((env) => env.name)}
            open={showReleaseTrainEnvs}
            onCancel={handleClose}
            onSubmit={confirmReleaseTrainFunction}
            envSelectionDialog={false}
        />
    );

    return (
        <div className="product_version">
            <h1 className="environment_name">{'Product Version Page'}</h1>
            {dialog}
            {tagsResponse.response.tagData.length > 0 ? (
                <div className="space_apart_row">
                    <div className="dropdown_div">
                        <select
                            onChange={onChangeTag}
                            className="drop_down"
                            data-testid="drop_down"
                            value={selectedTag}>
                            <option value="default" disabled>
                                Select a Tag
                            </option>
                            {tagsResponse.response.tagData.map((tag) => (
                                <option value={tag.commitId} key={tag.tag}>
                                    {tag.tag.slice(10)}
                                </option>
                            ))}
                        </select>
                        <select className="env_drop_down" onChange={changeEnv} value={environment}>
                            <option value="default" disabled>
                                Select an Environment or Environment Group
                            </option>
                            {envList.map((env) => (
                                <option value={env} key={env}>
                                    {env}
                                </option>
                            ))}
                        </select>
                    </div>
                    <Button
                        label={'Run Release Train'}
                        className="release_train_button"
                        onClick={openDialog}
                        highlightEffect={false}
                    />
                </div>
            ) : (
                <div />
            )}
            <div>
                {showSummary ? (
                    <div className="table_padding">
                        <TableFiltered productSummary={productSummaries} teams={teams} />
                    </div>
                ) : (
                    <div className="page_description">
                        {
                            'This page shows the version of the product for the selected environment based on tags to the repository. If there are no tags, then no data can be shown.'
                        }
                    </div>
                )}
            </div>
        </div>
    );
};
