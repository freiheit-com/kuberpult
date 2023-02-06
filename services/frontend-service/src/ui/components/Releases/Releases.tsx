import classNames from 'classnames';
import { Release } from '../../../api/api';
import { useReleasesForApp } from '../../utils/store';
import { ReleaseCardMini } from '../ReleaseCardMini/ReleaseCardMini';
import './Releases.scss';

export type ReleasesProps = {
    className?: string;
    app: string;
};

const dateFormat = (date: Date) => {
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    return `${months[date.getMonth()]} ${date.getDate()}, ${date.getFullYear()}`;
};

const getReleasesForAppGroupByDate = (releases: Array<Release>) => {
    if (releases === undefined) {
        return [];
    }
    const releaseGroupedByCreatedAt = releases.reduce((previousRelease: Release, curRelease: Release) => {
        (previousRelease[curRelease.createdAt?.toDateString()] =
            previousRelease[curRelease.createdAt?.toDateString()] || []).push(curRelease);
        return previousRelease;
    }, {});
    const rel: Array<Array<Release>> = [];
    for (const [, value] of Object.entries(releaseGroupedByCreatedAt)) {
        rel.push(value);
    }
    return rel;
};

export const Releases: React.FC<ReleasesProps> = (props) => {
    const { app, className } = props;
    const releases = useReleasesForApp(app);
    const rel = getReleasesForAppGroupByDate(releases);

    return (
        <div className={classNames('timeline', className)}>
            <h1 className={classNames('app_name', className)}>{'Releases | ' + app}</h1>
            {rel.map((release) => (
                <div key={release[0].version} className={classNames('container right', className)}>
                    <div className={classNames('release_date', className)}>{dateFormat(release[0].createdAt)}</div>
                    {release.map((rele) => (
                        <div key={rele.version} className={classNames('content', className)}>
                            <ReleaseCardMini app={app} version={rele.version} />
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
};
