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
import {
    addAction,
    AppDetailsResponse,
    AppDetailsState,
    EnvironmentGroupExtended,
    getAppDetails,
    showSnackbarError,
    showSnackbarWarn,
    updateAppDetails,
    useAppDetailsForApp,
    useCurrentlyExistsAtGroup,
    useMinorsForApp,
    useNavigateWithSearchParams,
} from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import { DeleteWhite, HistoryWhite } from '../../../images';
import { Environment, OverviewApplication, UndeploySummary } from '../../../api/api';
import * as React from 'react';
import { useCallback, useState } from 'react';
import { AppLockSummary } from '../chip/EnvironmentGroupChip';
import { WarningBoxes } from './Warnings';
import { DotsMenu, DotsMenuButton } from './DotsMenu';
import { EnvSelectionDialog } from '../SelectionDialog/SelectionDialogs';
import { AuthHeader, useAzureAuthSub } from '../../utils/AzureAuthProvider';
import { SmallSpinner } from '../Spinner/Spinner';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { Button } from '../button';
import { useSearchParams } from 'react-router-dom';
import { Tooltip } from '../tooltip/tooltip';

// number of releases on home. based on design
// we could update this dynamically based on viewport width
const numberOfDisplayedReleasesOnHome = 6;

const getReleasesToDisplay = (
    deployedReleases: number[],
    allReleases: number[],
    minorReleases: number[],
    ignoreMinors: boolean
): number[] => {
    if (ignoreMinors) {
        allReleases = allReleases.filter(
            (version) => !minorReleases.includes(version) || deployedReleases.includes(version)
        );
    }
    // all deployed releases are important and the latest release is also important
    const importantReleases = deployedReleases.includes(allReleases[0])
        ? deployedReleases
        : [allReleases[0], ...deployedReleases];
    // number of remaining releases to get from history
    const numOfTrailingReleases = numberOfDisplayedReleasesOnHome - importantReleases.length;
    // find the index of the last deployed release e.g. Prod (or -1 when there's no deployed releases)
    const oldestImportantReleaseIndex = importantReleases.length
        ? allReleases.findIndex((version) => version === importantReleases.slice(-1)[0])
        : -1;
    // take the deployed releases + a slice from the oldest element (or very first, see above) with total length 6
    return [
        ...importantReleases,
        ...allReleases.slice(oldestImportantReleaseIndex + 1, oldestImportantReleaseIndex + numOfTrailingReleases + 1),
    ];
};

function getNumberOfReleasesBetween(releases: number[], higherVersion: number, lowerVersion: number): number {
    // diff = index of lower version (older release) - index of higher version (newer release) - 1
    return releases.findIndex((ver) => ver === lowerVersion) - releases.findIndex((ver) => ver === higherVersion) - 1;
}

export const DiffElement: React.FC<{ diff: number; title: string; navCallback: () => void }> = ({
    diff,
    title,
    navCallback,
}) => (
    <div
        className="service-lane__diff--container"
        title={title}
        onClick={navCallback}
        data-testid="hidden-commits-button">
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--number">{diff}</div>
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
    </div>
);

const deriveUndeployMessage = (undeploySummary: UndeploySummary | undefined): string | undefined => {
    switch (undeploySummary) {
        case UndeploySummary.UNDEPLOY:
            return 'Delete Forever';
        case UndeploySummary.NORMAL:
            return 'Prepare Undeploy Release';
        case UndeploySummary.MIXED:
            return undefined;
        default:
            return undefined;
    }
};

export const ServiceLane: React.FC<{
    application: OverviewApplication;
    hideMinors: boolean;
}> = (props) => {
    const { application, hideMinors } = props;
    const { authHeader } = useAzureAuthSub((auth) => auth);

    const appDetails = useAppDetailsForApp(application.name);
    const componentRef: React.MutableRefObject<any> = React.useRef();
    const searchParams = useSearchParams();
    React.useEffect(() => {
        const handleScroll = (): void => {
            getAppDetailsIfInView(componentRef, appDetails, authHeader, application.name);
        };
        handleScroll();
        if (document.getElementsByClassName('mdc-drawer-app-content').length !== 0) {
            document.getElementsByClassName('mdc-drawer-app-content')[0].addEventListener('scroll', handleScroll);
            return () => {
                document
                    .getElementsByClassName('mdc-drawer-app-content')[0]
                    .removeEventListener('scroll', handleScroll);
            };
        }
    }, [appDetails, application, authHeader, searchParams]);
    return (
        <div ref={componentRef}>
            <GeneralServiceLane
                application={application}
                hideMinors={hideMinors}
                allAppData={appDetails}
                key={application.name}></GeneralServiceLane>
        </div>
    );
};

function getAppDetailsIfInView(
    componentRef: React.MutableRefObject<any>,
    appDetails: AppDetailsResponse,
    authHeader: AuthHeader,
    appName: string
): void {
    if (componentRef.current !== null) {
        const rect = componentRef.current.getBoundingClientRect();
        if (rect.top >= 0 && rect.bottom <= window.innerHeight) {
            if (appDetails.appDetailState === AppDetailsState.NOTREQUESTED) {
                getAppDetails(appName, authHeader);
            }
        }
    }
}

export const GeneralServiceLane: React.FC<{
    application: OverviewApplication;
    hideMinors: boolean;
    allAppData: AppDetailsResponse;
}> = (props) => {
    const onReload = useCallback(() => {
        const details = updateAppDetails.get();
        details[props.application.name] = {
            details: undefined,
            appDetailState: AppDetailsState.NOTREQUESTED,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        };
        updateAppDetails.set(details);
    }, [props.application.name]);

    let buttonClassName: string;

    switch (props.allAppData.appDetailState) {
        case AppDetailsState.ERROR: {
            buttonClassName = 'servicelane__reload__error';
            break;
        }
        case AppDetailsState.NOTFOUND: {
            buttonClassName = 'servicelane__reload__warn';
            break;
        }
        default: {
            buttonClassName = 'servicelane__reload';
        }
    }

    const reloadButton = (
        <Button
            id={props.application.name + '-reloadButton'}
            className={buttonClassName + ' mdc-button--unelevated'}
            label={'⟳'}
            highlightEffect={false}
            onClick={onReload}
        />
    );
    if (props.allAppData.appDetailState === AppDetailsState.READY) {
        return (
            <ReadyServiceLane
                application={props.application}
                hideMinors={props.hideMinors}
                allAppData={props.allAppData}></ReadyServiceLane>
        );
    } else if (props.allAppData.appDetailState === AppDetailsState.NOTFOUND) {
        return <NoDataServiceLane application={props.application} reloadButton={reloadButton}></NoDataServiceLane>;
    } else if (props.allAppData.appDetailState === AppDetailsState.NOTREQUESTED) {
        return <NotRequestedServiceLane application={props.application}></NotRequestedServiceLane>;
    } else if (props.allAppData.appDetailState === AppDetailsState.ERROR) {
        return (
            <ErrorServiceLane
                application={props.application}
                reloadButton={reloadButton}
                errorMessage={
                    props.allAppData.errorMessage && props.allAppData.errorMessage !== ''
                        ? props.allAppData.errorMessage
                        : 'no error message was provided.'
                }></ErrorServiceLane>
        );
    } else if (props.allAppData.appDetailState === AppDetailsState.LOADING) {
        return <LoadingServiceLane application={props.application}></LoadingServiceLane>;
    }
    return <LoadingServiceLane application={props.application}></LoadingServiceLane>;
};

const ServiceLaneHeaderData: React.FC<{
    application: OverviewApplication;
}> = (props) => (
    <div className={'service-lane-name'}>
        <span title={'team name'}>{props.application.team ? props.application.team : '<No Team> '} </span>
        {' | '} <span title={'app name'}> {props.application.name}</span>
    </div>
);

export const NoDataServiceLane: React.FC<{
    application: OverviewApplication;
    reloadButton: JSX.Element;
}> = (props) => (
    <div className="service-lane">
        <Tooltip
            id={props.application.name}
            tooltipContent={
                <span>Kuberpult could not find any data for this application. Try reloading the application.</span>
            }>
            <div className="service-lane__header__warn tooltip">
                <div className="service-lane-wrapper">
                    <ServiceLaneHeaderData application={props.application}></ServiceLaneHeaderData>
                </div>
                <div className="service-lane-wrapper">
                    <div>{props.reloadButton}</div>
                </div>

                {/*<div className="service__actions__">{dotsMenu}</div>*/}
            </div>
        </Tooltip>
        <div className="service__releases">{}</div>
    </div>
);
export const LoadingServiceLane: React.FC<{
    application: OverviewApplication;
}> = (props) => (
    <div className="service-lane">
        <div className="service-lane__header">
            <div className="service-lane-wrapper">
                <ServiceLaneHeaderData application={props.application}></ServiceLaneHeaderData>
                <SmallSpinner appName={props.application.name} key={props.application.name} />
            </div>
        </div>
        <div className="service__releases" key={props.application.name + '-' + props.application.team}></div>
    </div>
);
export const NotRequestedServiceLane: React.FC<{
    application: OverviewApplication;
}> = (props) => (
    <div className="service-lane">
        <div className="service-lane__header__not_requested">
            <div className="service-lane-wrapper">
                <ServiceLaneHeaderData application={props.application}></ServiceLaneHeaderData>
            </div>
        </div>
        <div className="service__releases" key={props.application.name + '-' + props.application.team}></div>
    </div>
);

export const ErrorServiceLane: React.FC<{
    application: OverviewApplication;
    reloadButton: JSX.Element;
    errorMessage: string;
}> = (props) => (
    <div className="service-lane">
        <Tooltip
            id={props.application.name}
            tooltipContent={
                <span>
                    {'Kuberpult got an error retrieving the information for this app. Error: ' + props.errorMessage}
                </span>
            }>
            <div className="service-lane__header__error">
                <div className="service-lane-wrapper">
                    <ServiceLaneHeaderData application={props.application}></ServiceLaneHeaderData>
                </div>
                <div className="service-lane-wrapper">
                    <div>{props.reloadButton}</div>
                </div>
                {/*<div className="service__actions__">{dotsMenu}</div>*/}
            </div>
        </Tooltip>
        <div className="service__releases">{}</div>
    </div>
);

export const ReadyServiceLane: React.FC<{
    application: OverviewApplication;
    hideMinors: boolean;
    allAppData: AppDetailsResponse;
}> = (props) => {
    const { application, hideMinors } = props;
    const { navCallback } = useNavigateWithSearchParams('releasehistory/' + application.name);
    const appDetails = props.allAppData.details;
    const allReleases = [...new Set(appDetails?.application?.releases.map((d) => d.version))];
    const deployments = appDetails?.deployments;
    const allDeployedReleaseNumbers = [];
    for (const prop in deployments) {
        allDeployedReleaseNumbers.push(deployments[prop].version);
    }
    const deployedReleases = [...new Set(allDeployedReleaseNumbers.map((v) => v).sort((n1, n2) => n2 - n1))];

    const prepareUndeployOrUndeploy = React.useCallback(() => {
        if (allReleases.length === 0) {
            // if there are no releases, we have to first create the undeploy release
            // and then undeploy:
            addAction({
                action: {
                    $case: 'prepareUndeploy',
                    prepareUndeploy: { application: application.name },
                },
            });
            addAction({
                action: {
                    $case: 'undeploy',
                    undeploy: { application: application.name },
                },
            });
            return;
        }
        switch (appDetails?.application?.undeploySummary) {
            case UndeploySummary.UNDEPLOY:
                addAction({
                    action: {
                        $case: 'undeploy',
                        undeploy: { application: application.name },
                    },
                });
                break;
            case UndeploySummary.NORMAL:
                addAction({
                    action: {
                        $case: 'prepareUndeploy',
                        prepareUndeploy: { application: application.name },
                    },
                });
                break;
            case UndeploySummary.MIXED:
                showSnackbarError('Internal Error: Cannot prepare to undeploy or actual undeploy in mixed state.');
                break;
            default:
                showSnackbarError('Internal Error: Cannot prepare to undeploy or actual undeploy in unknown state.');
                break;
        }
    }, [application.name, appDetails?.application?.undeploySummary, allReleases.length]);
    let minorReleases = useMinorsForApp(application.name);
    if (!minorReleases) {
        minorReleases = [];
    }
    const prepareUndeployOrUndeployText = deriveUndeployMessage(appDetails?.application?.undeploySummary);
    const releases = [
        ...new Set(
            getReleasesToDisplay(deployedReleases, allReleases, minorReleases, hideMinors).sort((n1, n2) => n2 - n1)
        ),
    ];
    const releases_lane =
        !!releases &&
        releases.map((rel, index) => {
            // diff is releases between current card and the next.
            // for the last card, diff is number of remaining releases in history
            const diff =
                index < releases.length - 1
                    ? getNumberOfReleasesBetween(allReleases, rel, releases[index + 1])
                    : getNumberOfReleasesBetween(allReleases, rel, allReleases.slice(-1)[0]) + 1;
            return (
                <div key={application.name + '-' + rel} className="service-lane__diff">
                    <ReleaseCard app={application.name} version={rel} key={application.name + '-' + rel} />
                    {!!diff && (
                        <DiffElement
                            diff={diff}
                            title={'There are ' + diff + ' more releases hidden. Click me to view more.'}
                            navCallback={navCallback}
                        />
                    )}
                </div>
            );
        });

    const envs: Environment[] = useCurrentlyExistsAtGroup(application.name).flatMap(
        (envGroup: EnvironmentGroupExtended) => envGroup.environments
    );

    const [showEnvSelectionDialog, setShowEnvSelectionDialog] = useState(false);

    const handleClose = useCallback(() => {
        setShowEnvSelectionDialog(false);
    }, []);
    const confirmEnvAppDelete = useCallback(
        (selectedEnvs: string[]) => {
            if (selectedEnvs.length === envs.length) {
                showSnackbarWarn("If you want to delete all environments, use 'prepare undeploy'");
                setShowEnvSelectionDialog(false);
                return;
            }
            selectedEnvs.forEach((env) => {
                addAction({
                    action: {
                        $case: 'deleteEnvFromApp',
                        deleteEnvFromApp: { application: application.name, environment: env },
                    },
                });
            });
            setShowEnvSelectionDialog(false);
        },
        [application.name, envs]
    );
    const onReload = useCallback(() => {
        const details = updateAppDetails.get();
        details[application.name] = {
            details: undefined,
            appDetailState: AppDetailsState.NOTREQUESTED,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        };
        updateAppDetails.set(details);
    }, [application.name]);
    const buttons: DotsMenuButton[] = [
        {
            label: 'View History',
            icon: <HistoryWhite />,
            onClick: (): void => {
                navCallback();
            },
        },
        {
            label: 'Remove environment from app',
            icon: <DeleteWhite />,
            onClick: (): void => {
                setShowEnvSelectionDialog(true);
            },
        },
    ];
    if (prepareUndeployOrUndeployText) {
        buttons.push({
            label: prepareUndeployOrUndeployText,
            onClick: prepareUndeployOrUndeploy,
            icon: <DeleteWhite />,
        });
    }

    const dotsMenu = <DotsMenu buttons={buttons} />;
    const appLocks = Object.values(appDetails?.appLocks ? appDetails.appLocks : []);
    const teamLocks = Object.values(appDetails?.teamLocks ? appDetails.teamLocks : []);
    const dialog = (
        <EnvSelectionDialog
            environments={envs.map((e) => e.name)}
            open={showEnvSelectionDialog}
            onSubmit={confirmEnvAppDelete}
            onCancel={handleClose}
            envSelectionDialog={true}
        />
    );

    const reloadButton = (
        <Button
            id={application.name + '-reloadButton'}
            className="servicelane__reload mdc-button--unelevated"
            label={'⟳'}
            highlightEffect={false}
            onClick={onReload}
        />
    );

    return (
        <div className="service-lane">
            {dialog}
            <div className="service-lane__header">
                <div className="service-lane-wrapper">
                    {appLocks.length + teamLocks.length >= 1 && (
                        <div className={'test-app-lock-summary'}>
                            <AppLockSummary app={application.name} numLocks={appLocks.length + teamLocks.length} />
                        </div>
                    )}
                    <ServiceLaneHeaderData application={props.application}></ServiceLaneHeaderData>
                </div>
                <div className="service-lane-wrapper">
                    <div>{reloadButton}</div>
                    {props.allAppData.updatedAt && (
                        <div className="service-lane__date">
                            <span>Updated </span>
                            <FormattedDate className={'date'} createdAt={props.allAppData.updatedAt} />
                        </div>
                    )}
                    <div className="service__actions__">{dotsMenu}</div>
                </div>
                {/*<div className="service__actions__">{dotsMenu}</div>*/}
            </div>
            <div className="service__warnings">
                <WarningBoxes application={appDetails?.application} />
            </div>
            <div className="service__releases">{releases_lane}</div>
        </div>
    );
};
