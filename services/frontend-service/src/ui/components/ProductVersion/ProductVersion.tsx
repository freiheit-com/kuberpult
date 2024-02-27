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

Copyright 2023 freiheit.com*/
import './ProductVersion.scss';
import * as React from 'react';
import {
    refreshTags,
    useTags,
    getSummary,
    useSummaryDisplay,
    useEnvironmentGroups,
    useEnvironments,
} from '../../utils/store';
import { DisplayManifestLink, DisplaySourceLink } from '../../utils/Links';
import { Spinner } from '../Spinner/Spinner';
import { EnvironmentGroup, ProductSummary } from '../../../api/api';
import { useSearchParams } from 'react-router-dom';
import { Button } from '../button';
import { EnvSelectionDialog } from '../ServiceLane/EnvSelectionDialog';

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
    const summaryResponse = useSummaryDisplay();
    const teams = (searchParams.get('teams') || '').split(',').filter((val) => val !== '');
    const openClose = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            const env = splitCombinedGroupName(environment);
            getSummary(e.target.value, env[0], env[1]);
            setSelectedTag(e.target.value);
            searchParams.set('tag', e.target.value);
            setSearchParams(searchParams);
        },
        [environment, searchParams, setSearchParams]
    );
    const [selectedTag, setSelectedTag] = React.useState('');
    const envsList = useEnvironments();

    const tagsResponse = useTags();
    const changeEnv = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            const env = splitCombinedGroupName(e.target.value);
            searchParams.set('env', e.target.value);
            searchParams.set('tag', selectedTag);
            setEnvironment(e.target.value);
            setSearchParams(searchParams);
            getSummary(selectedTag, env[0], env[1]);
        },
        [setSearchParams, searchParams, selectedTag]
    );
    const [displaySummary, setDisplayVersion] = React.useState(false);
    const [showReleaseTrainEnvs, setShowReleaseTrainEnvs] = React.useState(false);
    const handleClose = React.useCallback(() => {
        setShowReleaseTrainEnvs(false);
    }, []);
    const [showButton, setShowButton] = React.useState(false);
    React.useEffect(() => {
        if (localStorage.getItem('testing') !== null) {
            setShowButton(true);
        } else {
            setShowButton(false);
        }
    }, []);
    const openDialog = React.useCallback(() => {
        setShowReleaseTrainEnvs(true);
    }, []);
    const confirmReleaseTrainFunction = React.useCallback((selectedEnvs: string[]) => {
        selectedEnvs.forEach((env) => {
            // addAction() call added in DSN-3ZRNTG
        });
        return;
    }, []);

    React.useEffect(() => {
        if (tagsResponse.response.tagData.length > 0) {
            const env = splitCombinedGroupName(environment);
            if (searchParams.get('tag') === '') {
                setSelectedTag(tagsResponse.response.tagData[0].commitId);
                searchParams.set('tag', tagsResponse.response.tagData[0].commitId);
                setSearchParams(searchParams);
                getSummary(tagsResponse.response.tagData[0].commitId, env[0], env[1]);
            } else {
                getSummary(searchParams.get('tag') || tagsResponse.response.tagData[0].commitId, env[0], env[1]);
            }
            setDisplayVersion(true);
        }
    }, [tagsResponse, envGroupResponse, environment, searchParams, setSearchParams]);
    if (!tagsResponse.tagsReady) {
        return <Spinner message="Loading Git Tags" />;
    }
    if (!summaryResponse.summaryReady) {
        return <Spinner message="Loading Production Version" />;
    }

    const dialog = (
        <EnvSelectionDialog
            environments={envsList
                .filter((env, index) => environment === env.config?.upstream?.environment)
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
                            onChange={openClose}
                            onSelect={openClose}
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
                    {showButton ? (
                        <Button label={'Run Release Train'} className="release_train_button" onClick={openDialog} />
                    ) : (
                        <></>
                    )}
                </div>
            ) : (
                <div />
            )}
            <div>
                {displaySummary ? (
                    <div className="table_padding">
                        <TableFiltered productSummary={summaryResponse.response.productSummary} teams={teams} />
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
