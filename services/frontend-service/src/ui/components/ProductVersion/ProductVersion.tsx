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
import { refreshTags, useTags, getSummary, useSummaryDisplay } from '../../utils/store';
import { DisplayManifestLink, DisplaySourceLink } from '../../utils/Links';
import { Spinner } from '../Spinner/Spinner';
import { ProductSummary } from '../../../api/api';

export type ProductVersionProps = {
    environment: string;
};

export const ProductVersion: React.FC<ProductVersionProps> = (props) => {
    React.useEffect(() => {
        setShowTagsSpinner(true);
        refreshTags();
    }, []);
    const { environment } = props;
    const summaryResponse = useSummaryDisplay();
    const [open, setOpen] = React.useState(false);
    const openClose = React.useCallback(
        (e: React.ChangeEvent<HTMLSelectElement>) => {
            setShowSummarySpinner(true);
            getSummary(e.target.value, environment);
            setOpen(!open);
        },
        [open, setOpen, environment]
    );
    const [showTagsSpinner, setShowTagsSpinner] = React.useState(false);
    const [showSummarySpinner, setShowSummarySpinner] = React.useState(false);
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
    const [displaySummary, setDisplayVersion] = React.useState(false);

    React.useEffect(() => {
        if (tagsResponse.response.tagData.length > 0) {
            setShowSummarySpinner(true);
            getSummary(tagsResponse.response.tagData[0].commitId, environment);
            setDisplayVersion(true);
        }
    }, [tagsResponse, environment]);
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
            <h1 className="environment_name">{'Product Version for ' + environment}</h1>
            <div className="dropdown_div">
                <select onChange={openClose} onSelect={openClose} className="drop_down" data-testid="drop_down">
                    <option value="default" disabled>
                        Select a Tag
                    </option>
                    {tagsResponse.response.tagData.map((tag) => (
                        <option value={tag.commitId} key={tag.tag}>
                            {tag.tag.slice(10)}
                        </option>
                    ))}
                </select>
            </div>
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
