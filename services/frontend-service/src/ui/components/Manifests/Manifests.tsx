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
                <h2> Backend returned an empty manifest for {EnvironmentName}.</h2>
                <hr />
            </div>
        );
    }

    return (
        <div>
            <h2> Environment: {EnvironmentName} </h2>
            <div id={'manifest-' + EnvironmentName} className={'manifest-container'}>
                <pre>{manifest}</pre>
            </div>
            <hr />
        </div>
    );
};
