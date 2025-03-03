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
import { EslFailedItem, GetFailedEslsResponse } from '../../../api/api';
import { Button } from '../button';
import classNames from 'classnames';
import { useApi } from '../../utils/GrpcApi';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import { removeFromFailedEsls, showSnackbarError, showSnackbarSuccess } from '../../utils/store';
import SkipNextIcon from '@material-ui/icons/SkipNext';
import CachedIcon from '@material-ui/icons/Cached';

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
                .then(() => removeFromFailedEsls(eslVersion))
                .then(() => {
                    showSnackbarSuccess('Retried transformer with ID ' + eslVersion + '.');
                })
                .catch((e) => {
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
            className="lock-display-info lock-action service-action--delete"
            onClick={onClickRetry}
            key={'button-first-key'}
            icon={<CachedIcon />}
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
                .then(() => removeFromFailedEsls(eslVersion))
                .then(() => {
                    showSnackbarSuccess('Skipped transformer with ID ' + eslVersion + '.');
                })
                .catch((e) => {
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
            className="lock-display-info lock-action service-action--delete"
            key={'button-first-key'}
            icon={<SkipNextIcon />}
            highlightEffect={false}
        />
    );
};

type EslWarningsProps = {
    failedEsls: GetFailedEslsResponse | undefined;
    onClick?: { (): void };
};

const headers = ['Date', 'ID', 'Type', 'Reason', '', ''];

export const EslWarnings: React.FC<EslWarningsProps> = (props) => {
    const failedEslsResponse = props.failedEsls;
    const onClick = props.onClick;
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
    React.useEffect(() => {
        const currSroll = sessionStorage.getItem('scrollPosition');
        if (currSroll) {
            document.getElementsByClassName('mdc-drawer-app-content')[0].scrollTo({
                top: parseInt(currSroll) + 100,
                behavior: 'smooth',
            }); //100 is a slight nudge
            sessionStorage.removeItem('scrollPosition');
        }
    }, []);

    if (failedEslsResponse === undefined) {
        return (
            <div>
                <main className="main-content esl-warnings-page">Backend returned empty response</main>
            </div>
        );
    }

    const loadMoreButton = failedEslsResponse.loadMore ? (
        <div className="load-more-button-container">
            <button className="mdc-button button-main env-card-deploy-btn mdc-button--unelevated" onClick={onClick}>
                Load more
            </button>
        </div>
    ) : (
        <div></div>
    );

    return (
        <div>
            <main className="main-content esl-warnings-page">
                <h1>Failed ESL Event List: </h1>
                <div>
                    This page shows all events that could not be processed, and therefore were never written to the
                    manifest repo. Any operation in kuberpult is an event, like creating a lock or running a release
                    train.
                </div>
                <div className={classNames('mdc-data-table', 'locks-table')}>
                    <select className={'select-timezone'} value={timezone} onChange={handleChangeTimezone}>
                        <option value="local">{localTimezone} Timezone</option>
                        <option value="UTC">UTC Timezone</option>
                    </select>
                    <div className="mdc-data-table__table-container">
                        <table className="mdc-data-table__table" aria-label="Dessert calories">
                            <thead>
                                <tr className="mdc-data-table__header-row">
                                    <th className="mdc-data-indicator" role="columnheader" scope="col">
                                        <div className="mdc-data-header-title">{'Failed Esl Events'}</div>
                                    </th>
                                </tr>
                                <tr className="mdc-data-table__header-row">
                                    <th
                                        className="mdc-data-indicator mdc-data-indicator--subheader"
                                        role="columnheader"
                                        scope="col">
                                        <div className="mdc-data-indicator-header">
                                            {headers.map((columnHeader, idx) => (
                                                <div
                                                    key={columnHeader + idx}
                                                    className="mdc-data-indicator-field"
                                                    style={columnHeader === 'Reason' ? { flexGrow: 2 } : {}}>
                                                    {columnHeader}
                                                </div>
                                            ))}
                                        </div>
                                    </th>
                                </tr>
                            </thead>
                            <tbody className="mdc-data-table__content">
                                <tr>
                                    <td>
                                        {failedEslsResponse.failedEsls.map((eslItem, _) => (
                                            <FailedEslDisplay
                                                key={'failed_esl_' + eslItem.transformerEslVersion}
                                                failedItem={eslItem}
                                                timezone={timezone}></FailedEslDisplay>
                                        ))}
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>
                {loadMoreButton}
            </main>
        </div>
    );
};

export const FailedEslDisplay: React.FC<{ failedItem: EslFailedItem; timezone: string }> = (props) => {
    const { failedItem, timezone } = props;

    const createdAt = useCallback(() => {
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
        return formatDate(failedItem.createdAt);
    }, [failedItem.createdAt, timezone]);

    return (
        <div className="lock-display">
            <div className="lock-display__table">
                <div className="lock-display-table">
                    <div className="lock-display-info">{createdAt()}</div>
                    <div className="lock-display-info">{failedItem.transformerEslVersion}</div>
                    <div className="lock-display-info">{failedItem.eventType}</div>
                    <div className="lock-display-info-size-limit">{failedItem.reason}</div>
                    <RetryButton eslVersion={failedItem.transformerEslVersion}></RetryButton>
                    <SkipButton eslVersion={failedItem.transformerEslVersion}></SkipButton>
                </div>
            </div>
        </div>
    );
};
