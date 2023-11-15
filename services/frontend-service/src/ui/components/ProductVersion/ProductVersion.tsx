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
import { refreshTags, useTags } from '../../utils/store';
import { useApi } from '../../utils/GrpcApi';

export type ProductVersionProps = {
    environment: string;
};
export const ProductVersion: React.FC<ProductVersionProps> = (props) => {
    const api = useApi;
    React.useEffect(() => {
        refreshTags();
    }, [api]);
    const { environment } = props;
    const [open, setOpen] = React.useState(false);
    const openClose = React.useCallback(() => {
        setOpen(!open);
    }, [open, setOpen]);
    const tags = useTags();
    return (
        <div className="product_version">
            <h1 className="environment_name">{'Product Version for ' + environment}</h1>
            <div>
                <select onChange={openClose} onSelect={openClose} className="drop_down" data-testid="drop_down">
                    <option value="default" disabled selected>
                        Select a Tag
                    </option>
                    {tags.map((tag) => (
                        <option value={tag.tag} key={tag.tag}>
                            {tag.tag.slice(10)}
                        </option>
                    ))}
                </select>
            </div>
            <div className="page_description">
                {
                    'This page shows the version of the product for the selected environment based on tags to the repository. If there are no tags, then no data can be shown.'
                }
            </div>
        </div>
    );
};
