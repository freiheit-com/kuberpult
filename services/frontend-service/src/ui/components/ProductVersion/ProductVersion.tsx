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
import { refreshTags, useTags, getSummary, useSummaryDisplay, useEnvironmentGroups } from '../../utils/store';
import { DisplayManifestLink, DisplaySourceLink } from '../../utils/Links';
import { Spinner } from '../Spinner/Spinner';
import { ProductSummary } from '../../../api/api';
import { useSearchParams } from 'react-router-dom';

// splits up a string like "dev:dev-de" into ["dev", "dev-de"]
const splitCombinedGroupName = (envName: string): string[] => {
    const splitter = envName.split('/');
    if (splitter.length === 1) {
        return ['', splitter[0]];
    }
    return [splitter[1], ''];
};

export const ProductVersion: React.FC = () => {
    React.useEffect(() => {
        setShowTagsSpinner(true);
        refreshTags();
    }, []);
    const envGroupResponse = useEnvironmentGroups();
    const envList: string[] = [];
    for (let i = 0; i < envGroupResponse.length; i++) {
        envList.push(envGroupResponse[i].environmentGroupName);
        for (let j = 0; j < envGroupResponse[i].environments.length; j++) {
            envList.push(envGroupResponse[i].environmentGroupName + '/' + envGroupResponse[i].environments[j].name);
        }
    }
    const [searchParams, setSearchParams] = useSearchParams();
    const [environment, setEnvironment] = React.useState(searchParams.get('env') || envList[0]);
    const summaryResponse = useSummaryDisplay();
    const [open, setOpen] = React.useState(false);
    const openClose = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            setShowSummarySpinner(true);
            const env = splitCombinedGroupName(environment);
            getSummary(e.target.value, env[0], env[1]);
            setOpen(!open);
            setSelectedTag(e.target.value);
        },
        [open, setOpen, environment]
    );
    const [showTagsSpinner, setShowTagsSpinner] = React.useState(false);
    const [showSummarySpinner, setShowSummarySpinner] = React.useState(false);
    const [selectedTag, setSelectedTag] = React.useState('');
    var versionToDisplay = (app: ProductSummary): string => {
        if (app.displayVersion !== '') {
            return app.displayVersion;
        }
        if (app.commitId !== '') {
            return app.commitId;
        }
        return app.version;
    };

    const tagsResponse = useTags();
    const changeEnv = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            setShowSummarySpinner(true);
            const env = splitCombinedGroupName(e.target.value);
            searchParams.set('env', e.target.value);
            setEnvironment(e.target.value);
            setSearchParams(searchParams);
            getSummary(selectedTag, env[0], env[1]);
        },
        [setSearchParams, searchParams, selectedTag]
    );
    const [displaySummary, setDisplayVersion] = React.useState(false);

    React.useEffect(() => {
        if (tagsResponse.response.tagData.length > 0) {
            setShowSummarySpinner(true);
            const env = splitCombinedGroupName(environment);
            getSummary(tagsResponse.response.tagData[0].commitId, env[0], env[1]);
            setDisplayVersion(true);
            setSelectedTag(tagsResponse.response.tagData[0].commitId);
        }
    }, [tagsResponse, envGroupResponse, searchParams, environment]);
    React.useEffect(() => {
        if (tagsResponse.tagsReady) {
            setShowTagsSpinner(false);
        }
    }, [tagsResponse]);
    React.useEffect(() => {
        if (summaryResponse.summaryReady) {
            setShowSummarySpinner(false);
        }
    }, [summaryResponse]);
    if (showTagsSpinner) {
        return <Spinner message="Loading Tag Data" />;
    }
    if (showSummarySpinner) {
        return <Spinner message="Loading Summary Data" />;
    }

    return (
        <div className="product_version">
            <h1 className="environment_name">{'Product Version Page'}</h1>

            {tagsResponse.response.tagData.length > 0 ? (
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
            ) : (
                <div />
            )}
            <div>
                {displaySummary ? (
                    <div className="table_padding">
                        <table className="table">
                            <tbody>
                                <tr className="table_title">
                                    <th>App Name</th>
                                    <th>Version</th>
                                    <th>Environment</th>
                                    <th>ManifestRepoLink</th>
                                    <th>SourceRepoLink</th>
                                </tr>
                                {summaryResponse.response.productSummary.map((sum) => (
                                    <tr key={sum.app + sum.environment} className="table_data">
                                        <td>{sum.app}</td>
                                        <td>{versionToDisplay(sum)}</td>
                                        <td>{sum.environment}</td>
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
