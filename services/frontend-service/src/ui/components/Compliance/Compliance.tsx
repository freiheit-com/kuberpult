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
import React, { useCallback } from 'react';
import { Button } from '../button';
import { useApi } from '../../utils/GrpcApi';

export const Compliance: React.FC = () => {
    const api = useApi;

    const onClick = useCallback(() => {
        const content: string[] = [];
        api.overviewService()
            .StreamDeploymentHistory({})
            .subscribe({
                next: (res) => {
                    content.push(res.deployment);
                },
                complete: () => {
                    const filename = 'deployments.csv';
                    const file = new File(content, filename);
                    const anchor = document.createElement('a');
                    anchor.href = URL.createObjectURL(file);
                    anchor.download = filename;
                    anchor.click();
                    URL.revokeObjectURL(anchor.href);
                },
            });
    }, [api]);

    return (
        <div>
            <main className="main-content">
                <Button
                    onClick={onClick}
                    className={'button-main mdc-button--unelevated'}
                    label="Download CSV"
                    highlightEffect={false}
                />
            </main>
        </div>
    );
};
