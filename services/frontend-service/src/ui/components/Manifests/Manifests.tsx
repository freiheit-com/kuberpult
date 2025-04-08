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
import React from 'react';
// @ts-ignore
import yaml from 'js-yaml';

type ManifestProps = {
    Manifest: string;
    EnvironmentName: string;
};

export const Manifest: React.FC<ManifestProps> = (props) => {
    const manifest = props.Manifest;
    const EnvironmentName = props.EnvironmentName;

    if (manifest === '') {
        return (
            <div>
                <main className="main-content commit-page">Backend returned empty response</main>
            </div>
        );
    }
    return (
        <div>
            <h2> Environment: {EnvironmentName} </h2>
            <div id={'manifest-' + EnvironmentName} className={'manifest-container'}>
                <pre>{yaml.dump(manifest, { indent: 4 })}</pre>
            </div>
            <hr />
        </div>
    );
};
