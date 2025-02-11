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
import React, { useCallback, useState } from 'react';
import { GetFailedEslsResponse } from '../../../api/api';
import { Button } from '../button';
import classNames from 'classnames';
import { useApi } from '../../utils/GrpcApi';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import { getFailedEsls, showSnackbarError } from '../../utils/store';

type RetryButtonProps = {
    eslVersion: number;
    onClick?: { (): void };
};
export const RetryButton: React.FC<RetryButtonProps> = ({ eslVersion }) => {
    const { authHeader, authReady } = useAzureAuthSub((auth) => auth);
    const api = useApi;
    const onClickRetry = useCallback(() => {
        if (authReady) {
            api.gitService()
                .RetryFailedEvent({ eslversion: eslVersion }, authHeader)
                .then(() => {
                    getFailedEsls(authHeader);
                })
                .catch((e) => {
                    // eslint-disable-next-line no-console
                    console.error('error in retry request: ', e);
                    const GrpcErrorPermissionDenied = 7;
                    if (e.code === GrpcErrorPermissionDenied) {
                        showSnackbarError(e.message);
                    } else {
                        showSnackbarError('retry not successful. Please try again');
                    }
                });
        }
    }, [api, authHeader, authReady, eslVersion]);
    return (
        <Button
            onClick={onClickRetry}
            className={classNames('button-main', 'mdc-button--unelevated')}
            key={'button-first-key'}
            label="Retry"
            highlightEffect={false}
        />
    );
};

export const SkipButton: React.FC<RetryButtonProps> = ({ eslVersion }) => {
    const { authHeader, authReady } = useAzureAuthSub((auth) => auth);
    const api = useApi;
    const onClickSkip = useCallback(() => {
        if (authReady) {
            api.gitService()
                .SkipEslEvent({ eventEslVersion: eslVersion }, authHeader)
                .then(() => {
                    getFailedEsls(authHeader);
                })
                .catch((e) => {
                    // eslint-disable-next-line no-console
                    console.error('error in retry request: ', e);
                    const GrpcErrorPermissionDenied = 7;
                    if (e.code === GrpcErrorPermissionDenied) {
                        showSnackbarError(e.message);
                    } else {
                        showSnackbarError('skip not successful. Please try again');
                    }
                });
        }
    }, [api, authHeader, authReady, eslVersion]);
    return (
        <Button
            onClick={onClickSkip}
            className={classNames('button-main', 'mdc-button--unelevated')}
            key={'button-first-key'}
            label="Skip"
            highlightEffect={false}
        />
    );
};

type EslWarningsProps = {
    failedEsls: GetFailedEslsResponse | undefined;
};

export const EslWarnings: React.FC<EslWarningsProps> = (props) => {
    const failedEslsResponse = props.failedEsls;
    const [timezone, setTimezone] = useState<'UTC' | 'local'>('UTC');
    const localTimezone = Intl.DateTimeFormat()?.resolvedOptions()?.timeZone ?? 'Europe/Berlin';

    const handleChangeTimezone = React.useCallback(
        (event: React.ChangeEvent<HTMLSelectElement>) => {
            if (event.target.value === 'local' || event.target.value === 'UTC') {
                setTimezone(event.target.value);
            }
        },
        [setTimezone]
    );
    if (failedEslsResponse === undefined) {
        return (
            <div>
                <main className="main-content esl-warnings-page">Backend returned empty response</main>
            </div>
        );
    }
    const convertTimeZone = (
        date: Date,
        timeZoneFrom?: string | null, // default timezone is Local
        timeZoneTo?: string | null // default timezone is Local
    ): Date => {
        const dateFrom = !timeZoneFrom
            ? date
            : new Date(
                  date.toLocaleString('en-US', {
                      timeZone: timeZoneFrom,
                  })
              );
        const dateTo = !timeZoneTo
            ? date
            : new Date(
                  date.toLocaleString('en-US', {
                      timeZone: timeZoneTo,
                  })
              );
        return new Date(date.getTime() + dateTo.getTime() - dateFrom.getTime());
    };
    const dateToString = (date: Date, timeZone: string | null): string => {
        date = convertTimeZone(date, 'UTC', timeZone);
        const year = date.getUTCFullYear().toString().padStart(4, '0');
        const month = (date.getUTCMonth() + 1).toString().padStart(2, '0');
        const day = date.getUTCDate().toString().padStart(2, '0');
        const hours = date.getUTCHours().toString().padStart(2, '0');
        const minutes = date.getUTCMinutes().toString().padStart(2, '0');
        const seconds = date.getUTCSeconds().toString().padStart(2, '0');
        return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}`;
    };
    const formatDate = (date: Date | undefined): string => {
        if (!date) return '';
        if (timezone === 'local') {
            const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
            return dateToString(date, zone);
        }
        return dateToString(date, 'UTC');
    };

    return (
        <div>
            <main className="main-content esl-warnings-page">
                <h1>Failed ESL Event List: </h1>
                <div>
                    This page shows all events that could not be processed, and therefore were never written to the
                    manifest repo. Any operation in kuberpult is an event, like creating a lock or running a release
                    train.
                </div>
                <div>
                    <select className={'select-timezone'} value={timezone} onChange={handleChangeTimezone}>
                        <option value="local">{localTimezone} Timezone</option>
                        <option value="UTC">UTC Timezone</option>
                    </select>
                    <table className={'esls'} border={1}>
                        <thead>
                            <tr>
                                <th className={'EslVersion'}>EslVersion:</th>
                                <th className={'date'}>Date:</th>
                                <th className={'Event Type'}>Event Type:</th>
                                <th className={'Json'}>Json:</th>
                                <th className={'Reason'}>Reason:</th>
                                <th className={'TransformerEslVersion'}>TransformerEslVersion:</th>
                                <th className={'Retry'}>Retry:</th>
                                <th className={'Skip'}>Skip:</th>
                            </tr>
                        </thead>
                        <tbody>
                            {failedEslsResponse.failedEsls.map((eslItem, _) => {
                                const createdAt = formatDate(eslItem.createdAt);
                                return (
                                    <tr key={eslItem.eslVersion}>
                                        <td>{eslItem.eslVersion}</td>
                                        <td>{createdAt}</td>
                                        <td>{eslItem.eventType}</td>
                                        <td>{eslItem.json}</td>
                                        <td>{eslItem.reason}</td>
                                        <td>{eslItem.transformerEslVersion}</td>
                                        <td>
                                            <RetryButton eslVersion={eslItem.transformerEslVersion}></RetryButton>
                                        </td>
                                        <td>
                                            <SkipButton eslVersion={eslItem.transformerEslVersion}></SkipButton>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            </main>
        </div>
    );
};
