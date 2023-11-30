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
import { EnvironmentGroup, ProductSummary } from '../../../api/api';

export type ProductVersionProps = {
    environment: string;
};

const handleEnvironmentName = (envName: string): string[] => {
    const splitter = envName.split('/');
    if (splitter.length === 1) {
        return ['', splitter[0]];
    }
    return [splitter[1], ''];
};

const findFirstEnvGroup = (envName: string, envList: EnvironmentGroup[]): string => {
    for (let i = 0; i < envList.length; i++) {
        for (let j = 0; j < envList[i].environments.length; j++) {
            if (envList[i].environments[j].name === envName) {
                return envList[i].environmentGroupName + '/' + envName;
            }
        }
    }
    return envName;
};

export const ProductVersion: React.FC<ProductVersionProps> = (props) => {
    React.useEffect(() => {
        setShowTagsSpinner(true);
        refreshTags();
    }, []);
    const environment = React.useRef(props.environment);
    const summaryResponse = useSummaryDisplay();
    const [open, setOpen] = React.useState(false);
    const openClose = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            setShowSummarySpinner(true);
            const env = handleEnvironmentName(environment.current);
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

    const envList: string[] = [];
    const tagsResponse = useTags();
    const envGroupResponse = useEnvironmentGroups();
    for (let i = 0; i < envGroupResponse.length; i++) {
        envList.push(envGroupResponse[i].environmentGroupName);
        for (let j = 0; j < envGroupResponse[i].environments.length; j++) {
            envList.push(envGroupResponse[i].environmentGroupName + '/' + envGroupResponse[i].environments[j].name);
        }
    }
    const changeEnv = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            environment.current = e.target.value;
            setShowSummarySpinner(true);
            const env = handleEnvironmentName(environment.current);
            getSummary(selectedTag, env[0], env[1]);
        },
        [selectedTag, environment]
    );
    const [displaySummary, setDisplayVersion] = React.useState(false);

    React.useEffect(() => {
        if (tagsResponse.response.tagData.length > 0) {
            setShowSummarySpinner(true);
            getSummary(tagsResponse.response.tagData[0].commitId, environment.current, '');
            environment.current = findFirstEnvGroup(environment.current, envGroupResponse);
            setDisplayVersion(true);
            setSelectedTag(tagsResponse.response.tagData[0].commitId);
        }
    }, [tagsResponse, environment, envGroupResponse]);
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
                    <select className="env_drop_down" onChange={changeEnv} value={environment.current}>
                        <option value="default" disabled>
                            Select an Environment or environmentGroup
                        </option>
                        {envList.map((env) => (
                            <option value={env}>{env}</option>
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
                            <tr className="table_title">
                                <th>App Name</th>
                                <th>Version</th>
                                <th>ManifestRepoLink</th>
                                <th>SourceRepoLink</th>
                            </tr>
                            {summaryResponse.response.productSummary.map((sum) => (
                                <tr key={sum.app} className="table_data">
                                    <td>{sum.app}</td>
                                    <td>{versionToDisplay(sum)}</td>
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
