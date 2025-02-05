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
import React, { ChangeEvent, useCallback, useState } from 'react';
import { Button } from '../button';
import { useApi } from '../../utils/GrpcApi';
import { showSnackbarError, useEnvironmentGroups } from '../../utils/store';

export type ComplianceProps = {
    saveFile: (lines: string[]) => void;
};

export const Compliance: React.FC<ComplianceProps> = ({ saveFile }) => {
    const api = useApi;
    const [startDate, setStartDate] = useState<Date>();
    const [endDate, setEndDate] = useState<Date>();
    const [environment, setEnvironment] = useState('default');

    const onClick = useCallback(() => {
        if (environment === 'default') {
            showSnackbarError('Cannot download deployment history without an environment selected.');
            return;
        }
        if (!startDate || !endDate) {
            showSnackbarError('Cannot download deployment history without a start and end date.');
            return;
        }
        if (endDate < startDate) {
            showSnackbarError('Cannot have an end date that happens before the start date.');
            return;
        }

        const content: string[] = [];
        api.overviewService()
            .StreamDeploymentHistory({ startDate, endDate, environment: environment.split('/')[1] })
            .subscribe({
                next: (res) => {
                    content.push(res.deployment);
                },
                complete: () => saveFile(content),
            });
    }, [api, endDate, startDate, saveFile, environment]);

    const onStartDateChange = useCallback((e: ChangeEvent<HTMLInputElement>) => {
        setStartDate(e.target.valueAsDate ?? undefined);
    }, []);

    const onEndDateChange = useCallback((e: ChangeEvent<HTMLInputElement>) => {
        setEndDate(e.target.valueAsDate ?? undefined);
    }, []);

    const environments = useEnvironmentGroups().flatMap((group) =>
        group.environments.map((env) => `${group.environmentGroupName}/${env.name}`)
    );

    const onEnvChange = useCallback((e: ChangeEvent<HTMLSelectElement>) => {
        setEnvironment(e.target.value);
    }, []);

    return (
        <div>
            <main className="main-content compliance-content">
                <select className="env_drop_down" onChange={onEnvChange} value={environment}>
                    <option value="default" disabled>
                        Select an Environment
                    </option>
                    {environments.map((env) => (
                        <option value={env} key={env}>
                            {env}
                        </option>
                    ))}
                </select>

                <span>From:</span>
                <input
                    type="date"
                    id="start-date"
                    className="mdc-button mdc-button--outlined"
                    onChange={onStartDateChange}
                />
                <span>To:</span>
                <input
                    type="date"
                    id="end-date"
                    className="mdc-button mdc-button--outlined"
                    onChange={onEndDateChange}
                />

                <Button
                    onClick={onClick}
                    className="button-main env-card-deploy-btn mdc-button--unelevated"
                    label="Download Deployment History CSV"
                    highlightEffect={false}
                />
            </main>
        </div>
    );
};
