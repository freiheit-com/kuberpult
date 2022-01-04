/* eslint-disable */
import Long from 'long';
import { grpc } from '@improbable-eng/grpc-web';
import _m0 from 'protobufjs/minimal';
import { Empty } from '../google/protobuf/empty';
import { Observable } from 'rxjs';
import { BrowserHeaders } from 'browser-headers';
import { share } from 'rxjs/operators';
import { Timestamp } from '../google/protobuf/timestamp';

export const protobufPackage = 'api.v1';

export enum LockBehavior {
    Queue = 0,
    Fail = 1,
    Ignore = 2,
    UNRECOGNIZED = -1,
}

export function lockBehaviorFromJSON(object: any): LockBehavior {
    switch (object) {
        case 0:
        case 'Queue':
            return LockBehavior.Queue;
        case 1:
        case 'Fail':
            return LockBehavior.Fail;
        case 2:
        case 'Ignore':
            return LockBehavior.Ignore;
        case -1:
        case 'UNRECOGNIZED':
        default:
            return LockBehavior.UNRECOGNIZED;
    }
}

export function lockBehaviorToJSON(object: LockBehavior): string {
    switch (object) {
        case LockBehavior.Queue:
            return 'Queue';
        case LockBehavior.Fail:
            return 'Fail';
        case LockBehavior.Ignore:
            return 'Ignore';
        default:
            return 'UNKNOWN';
    }
}

export interface BatchRequest {
    actions: BatchAction[];
}

export interface BatchAction {
    action?:
        | { $case: 'createEnvironmentLock'; createEnvironmentLock: CreateEnvironmentLockRequest }
        | { $case: 'deleteEnvironmentLock'; deleteEnvironmentLock: DeleteEnvironmentLockRequest }
        | {
              $case: 'createEnvironmentApplicationLock';
              createEnvironmentApplicationLock: CreateEnvironmentApplicationLockRequest;
          }
        | {
              $case: 'deleteEnvironmentApplicationLock';
              deleteEnvironmentApplicationLock: DeleteEnvironmentApplicationLockRequest;
          }
        | { $case: 'deploy'; deploy: DeployRequest }
        | { $case: 'prepareUndeploy'; prepareUndeploy: PrepareUndeployRequest };
}

export interface CreateEnvironmentLockRequest {
    environment: string;
    lockId: string;
    message: string;
}

export interface DeleteEnvironmentLockRequest {
    environment: string;
    lockId: string;
}

export interface CreateEnvironmentApplicationLockRequest {
    environment: string;
    application: string;
    lockId: string;
    message: string;
}

export interface DeleteEnvironmentApplicationLockRequest {
    environment: string;
    application: string;
    lockId: string;
}

export interface DeployRequest {
    environment: string;
    application: string;
    version: number;
    /** @deprecated */
    ignoreAllLocks: boolean;
    lockBehavior: LockBehavior;
}

export interface PrepareUndeployRequest {
    application: string;
}

export interface ReleaseTrainRequest {
    environment: string;
}

export interface Lock {
    message: string;
    commit?: Commit;
}

export interface LockedError {
    environmentLocks: { [key: string]: Lock };
    environmentApplicationLocks: { [key: string]: Lock };
}

export interface LockedError_EnvironmentLocksEntry {
    key: string;
    value?: Lock;
}

export interface LockedError_EnvironmentApplicationLocksEntry {
    key: string;
    value?: Lock;
}

export interface CreateEnvironmentRequest {
    environment: string;
}

export interface GetOverviewRequest {}

export interface GetOverviewResponse {
    environments: { [key: string]: Environment };
    applications: { [key: string]: Application };
}

export interface GetOverviewResponse_EnvironmentsEntry {
    key: string;
    value?: Environment;
}

export interface GetOverviewResponse_ApplicationsEntry {
    key: string;
    value?: Application;
}

export interface Environment {
    name: string;
    config?: Environment_Config;
    locks: { [key: string]: Lock };
    applications: { [key: string]: Environment_Application };
}

export interface Environment_Config {
    upstream?: Environment_Config_Upstream;
}

export interface Environment_Config_Upstream {
    upstream?: { $case: 'environment'; environment: string } | { $case: 'latest'; latest: boolean };
}

export interface Environment_Application {
    name: string;
    version: number;
    locks: { [key: string]: Lock };
    queuedVersion: number;
    versionCommit?: Commit;
    undeployVersion: boolean;
}

export interface Environment_Application_LocksEntry {
    key: string;
    value?: Lock;
}

export interface Environment_LocksEntry {
    key: string;
    value?: Lock;
}

export interface Environment_ApplicationsEntry {
    key: string;
    value?: Environment_Application;
}

export interface Release {
    version: number;
    sourceCommitId: string;
    sourceAuthor: string;
    sourceMessage: string;
    commit?: Commit;
    undeployVersion: boolean;
}

export interface Application {
    name: string;
    releases: Release[];
}

export interface Commit {
    authorTime?: Date;
    authorName: string;
    authorEmail: string;
}

const baseBatchRequest: object = {};

export const BatchRequest = {
    encode(message: BatchRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        for (const v of message.actions) {
            BatchAction.encode(v!, writer.uint32(10).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): BatchRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseBatchRequest } as BatchRequest;
        message.actions = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.actions.push(BatchAction.decode(reader, reader.uint32()));
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): BatchRequest {
        const message = { ...baseBatchRequest } as BatchRequest;
        message.actions = [];
        if (object.actions !== undefined && object.actions !== null) {
            for (const e of object.actions) {
                message.actions.push(BatchAction.fromJSON(e));
            }
        }
        return message;
    },

    toJSON(message: BatchRequest): unknown {
        const obj: any = {};
        if (message.actions) {
            obj.actions = message.actions.map((e) => (e ? BatchAction.toJSON(e) : undefined));
        } else {
            obj.actions = [];
        }
        return obj;
    },

    fromPartial(object: DeepPartial<BatchRequest>): BatchRequest {
        const message = { ...baseBatchRequest } as BatchRequest;
        message.actions = [];
        if (object.actions !== undefined && object.actions !== null) {
            for (const e of object.actions) {
                message.actions.push(BatchAction.fromPartial(e));
            }
        }
        return message;
    },
};

const baseBatchAction: object = {};

export const BatchAction = {
    encode(message: BatchAction, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.action?.$case === 'createEnvironmentLock') {
            CreateEnvironmentLockRequest.encode(
                message.action.createEnvironmentLock,
                writer.uint32(10).fork()
            ).ldelim();
        }
        if (message.action?.$case === 'deleteEnvironmentLock') {
            DeleteEnvironmentLockRequest.encode(
                message.action.deleteEnvironmentLock,
                writer.uint32(18).fork()
            ).ldelim();
        }
        if (message.action?.$case === 'createEnvironmentApplicationLock') {
            CreateEnvironmentApplicationLockRequest.encode(
                message.action.createEnvironmentApplicationLock,
                writer.uint32(26).fork()
            ).ldelim();
        }
        if (message.action?.$case === 'deleteEnvironmentApplicationLock') {
            DeleteEnvironmentApplicationLockRequest.encode(
                message.action.deleteEnvironmentApplicationLock,
                writer.uint32(34).fork()
            ).ldelim();
        }
        if (message.action?.$case === 'deploy') {
            DeployRequest.encode(message.action.deploy, writer.uint32(42).fork()).ldelim();
        }
        if (message.action?.$case === 'prepareUndeploy') {
            PrepareUndeployRequest.encode(message.action.prepareUndeploy, writer.uint32(50).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): BatchAction {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseBatchAction } as BatchAction;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.action = {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: CreateEnvironmentLockRequest.decode(reader, reader.uint32()),
                    };
                    break;
                case 2:
                    message.action = {
                        $case: 'deleteEnvironmentLock',
                        deleteEnvironmentLock: DeleteEnvironmentLockRequest.decode(reader, reader.uint32()),
                    };
                    break;
                case 3:
                    message.action = {
                        $case: 'createEnvironmentApplicationLock',
                        createEnvironmentApplicationLock: CreateEnvironmentApplicationLockRequest.decode(
                            reader,
                            reader.uint32()
                        ),
                    };
                    break;
                case 4:
                    message.action = {
                        $case: 'deleteEnvironmentApplicationLock',
                        deleteEnvironmentApplicationLock: DeleteEnvironmentApplicationLockRequest.decode(
                            reader,
                            reader.uint32()
                        ),
                    };
                    break;
                case 5:
                    message.action = { $case: 'deploy', deploy: DeployRequest.decode(reader, reader.uint32()) };
                    break;
                case 6:
                    message.action = {
                        $case: 'prepareUndeploy',
                        prepareUndeploy: PrepareUndeployRequest.decode(reader, reader.uint32()),
                    };
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): BatchAction {
        const message = { ...baseBatchAction } as BatchAction;
        if (object.createEnvironmentLock !== undefined && object.createEnvironmentLock !== null) {
            message.action = {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: CreateEnvironmentLockRequest.fromJSON(object.createEnvironmentLock),
            };
        }
        if (object.deleteEnvironmentLock !== undefined && object.deleteEnvironmentLock !== null) {
            message.action = {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: DeleteEnvironmentLockRequest.fromJSON(object.deleteEnvironmentLock),
            };
        }
        if (object.createEnvironmentApplicationLock !== undefined && object.createEnvironmentApplicationLock !== null) {
            message.action = {
                $case: 'createEnvironmentApplicationLock',
                createEnvironmentApplicationLock: CreateEnvironmentApplicationLockRequest.fromJSON(
                    object.createEnvironmentApplicationLock
                ),
            };
        }
        if (object.deleteEnvironmentApplicationLock !== undefined && object.deleteEnvironmentApplicationLock !== null) {
            message.action = {
                $case: 'deleteEnvironmentApplicationLock',
                deleteEnvironmentApplicationLock: DeleteEnvironmentApplicationLockRequest.fromJSON(
                    object.deleteEnvironmentApplicationLock
                ),
            };
        }
        if (object.deploy !== undefined && object.deploy !== null) {
            message.action = { $case: 'deploy', deploy: DeployRequest.fromJSON(object.deploy) };
        }
        if (object.prepareUndeploy !== undefined && object.prepareUndeploy !== null) {
            message.action = {
                $case: 'prepareUndeploy',
                prepareUndeploy: PrepareUndeployRequest.fromJSON(object.prepareUndeploy),
            };
        }
        return message;
    },

    toJSON(message: BatchAction): unknown {
        const obj: any = {};
        message.action?.$case === 'createEnvironmentLock' &&
            (obj.createEnvironmentLock = message.action?.createEnvironmentLock
                ? CreateEnvironmentLockRequest.toJSON(message.action?.createEnvironmentLock)
                : undefined);
        message.action?.$case === 'deleteEnvironmentLock' &&
            (obj.deleteEnvironmentLock = message.action?.deleteEnvironmentLock
                ? DeleteEnvironmentLockRequest.toJSON(message.action?.deleteEnvironmentLock)
                : undefined);
        message.action?.$case === 'createEnvironmentApplicationLock' &&
            (obj.createEnvironmentApplicationLock = message.action?.createEnvironmentApplicationLock
                ? CreateEnvironmentApplicationLockRequest.toJSON(message.action?.createEnvironmentApplicationLock)
                : undefined);
        message.action?.$case === 'deleteEnvironmentApplicationLock' &&
            (obj.deleteEnvironmentApplicationLock = message.action?.deleteEnvironmentApplicationLock
                ? DeleteEnvironmentApplicationLockRequest.toJSON(message.action?.deleteEnvironmentApplicationLock)
                : undefined);
        message.action?.$case === 'deploy' &&
            (obj.deploy = message.action?.deploy ? DeployRequest.toJSON(message.action?.deploy) : undefined);
        message.action?.$case === 'prepareUndeploy' &&
            (obj.prepareUndeploy = message.action?.prepareUndeploy
                ? PrepareUndeployRequest.toJSON(message.action?.prepareUndeploy)
                : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<BatchAction>): BatchAction {
        const message = { ...baseBatchAction } as BatchAction;
        if (
            object.action?.$case === 'createEnvironmentLock' &&
            object.action?.createEnvironmentLock !== undefined &&
            object.action?.createEnvironmentLock !== null
        ) {
            message.action = {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: CreateEnvironmentLockRequest.fromPartial(object.action.createEnvironmentLock),
            };
        }
        if (
            object.action?.$case === 'deleteEnvironmentLock' &&
            object.action?.deleteEnvironmentLock !== undefined &&
            object.action?.deleteEnvironmentLock !== null
        ) {
            message.action = {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: DeleteEnvironmentLockRequest.fromPartial(object.action.deleteEnvironmentLock),
            };
        }
        if (
            object.action?.$case === 'createEnvironmentApplicationLock' &&
            object.action?.createEnvironmentApplicationLock !== undefined &&
            object.action?.createEnvironmentApplicationLock !== null
        ) {
            message.action = {
                $case: 'createEnvironmentApplicationLock',
                createEnvironmentApplicationLock: CreateEnvironmentApplicationLockRequest.fromPartial(
                    object.action.createEnvironmentApplicationLock
                ),
            };
        }
        if (
            object.action?.$case === 'deleteEnvironmentApplicationLock' &&
            object.action?.deleteEnvironmentApplicationLock !== undefined &&
            object.action?.deleteEnvironmentApplicationLock !== null
        ) {
            message.action = {
                $case: 'deleteEnvironmentApplicationLock',
                deleteEnvironmentApplicationLock: DeleteEnvironmentApplicationLockRequest.fromPartial(
                    object.action.deleteEnvironmentApplicationLock
                ),
            };
        }
        if (
            object.action?.$case === 'deploy' &&
            object.action?.deploy !== undefined &&
            object.action?.deploy !== null
        ) {
            message.action = { $case: 'deploy', deploy: DeployRequest.fromPartial(object.action.deploy) };
        }
        if (
            object.action?.$case === 'prepareUndeploy' &&
            object.action?.prepareUndeploy !== undefined &&
            object.action?.prepareUndeploy !== null
        ) {
            message.action = {
                $case: 'prepareUndeploy',
                prepareUndeploy: PrepareUndeployRequest.fromPartial(object.action.prepareUndeploy),
            };
        }
        return message;
    },
};

const baseCreateEnvironmentLockRequest: object = { environment: '', lockId: '', message: '' };

export const CreateEnvironmentLockRequest = {
    encode(message: CreateEnvironmentLockRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        if (message.lockId !== '') {
            writer.uint32(18).string(message.lockId);
        }
        if (message.message !== '') {
            writer.uint32(26).string(message.message);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): CreateEnvironmentLockRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseCreateEnvironmentLockRequest } as CreateEnvironmentLockRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                case 2:
                    message.lockId = reader.string();
                    break;
                case 3:
                    message.message = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): CreateEnvironmentLockRequest {
        const message = { ...baseCreateEnvironmentLockRequest } as CreateEnvironmentLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = String(object.lockId);
        }
        if (object.message !== undefined && object.message !== null) {
            message.message = String(object.message);
        }
        return message;
    },

    toJSON(message: CreateEnvironmentLockRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        message.lockId !== undefined && (obj.lockId = message.lockId);
        message.message !== undefined && (obj.message = message.message);
        return obj;
    },

    fromPartial(object: DeepPartial<CreateEnvironmentLockRequest>): CreateEnvironmentLockRequest {
        const message = { ...baseCreateEnvironmentLockRequest } as CreateEnvironmentLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = object.lockId;
        }
        if (object.message !== undefined && object.message !== null) {
            message.message = object.message;
        }
        return message;
    },
};

const baseDeleteEnvironmentLockRequest: object = { environment: '', lockId: '' };

export const DeleteEnvironmentLockRequest = {
    encode(message: DeleteEnvironmentLockRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        if (message.lockId !== '') {
            writer.uint32(18).string(message.lockId);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): DeleteEnvironmentLockRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseDeleteEnvironmentLockRequest } as DeleteEnvironmentLockRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                case 2:
                    message.lockId = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): DeleteEnvironmentLockRequest {
        const message = { ...baseDeleteEnvironmentLockRequest } as DeleteEnvironmentLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = String(object.lockId);
        }
        return message;
    },

    toJSON(message: DeleteEnvironmentLockRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        message.lockId !== undefined && (obj.lockId = message.lockId);
        return obj;
    },

    fromPartial(object: DeepPartial<DeleteEnvironmentLockRequest>): DeleteEnvironmentLockRequest {
        const message = { ...baseDeleteEnvironmentLockRequest } as DeleteEnvironmentLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = object.lockId;
        }
        return message;
    },
};

const baseCreateEnvironmentApplicationLockRequest: object = {
    environment: '',
    application: '',
    lockId: '',
    message: '',
};

export const CreateEnvironmentApplicationLockRequest = {
    encode(message: CreateEnvironmentApplicationLockRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        if (message.application !== '') {
            writer.uint32(18).string(message.application);
        }
        if (message.lockId !== '') {
            writer.uint32(26).string(message.lockId);
        }
        if (message.message !== '') {
            writer.uint32(34).string(message.message);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): CreateEnvironmentApplicationLockRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseCreateEnvironmentApplicationLockRequest } as CreateEnvironmentApplicationLockRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                case 2:
                    message.application = reader.string();
                    break;
                case 3:
                    message.lockId = reader.string();
                    break;
                case 4:
                    message.message = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): CreateEnvironmentApplicationLockRequest {
        const message = { ...baseCreateEnvironmentApplicationLockRequest } as CreateEnvironmentApplicationLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        if (object.application !== undefined && object.application !== null) {
            message.application = String(object.application);
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = String(object.lockId);
        }
        if (object.message !== undefined && object.message !== null) {
            message.message = String(object.message);
        }
        return message;
    },

    toJSON(message: CreateEnvironmentApplicationLockRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        message.application !== undefined && (obj.application = message.application);
        message.lockId !== undefined && (obj.lockId = message.lockId);
        message.message !== undefined && (obj.message = message.message);
        return obj;
    },

    fromPartial(object: DeepPartial<CreateEnvironmentApplicationLockRequest>): CreateEnvironmentApplicationLockRequest {
        const message = { ...baseCreateEnvironmentApplicationLockRequest } as CreateEnvironmentApplicationLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        if (object.application !== undefined && object.application !== null) {
            message.application = object.application;
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = object.lockId;
        }
        if (object.message !== undefined && object.message !== null) {
            message.message = object.message;
        }
        return message;
    },
};

const baseDeleteEnvironmentApplicationLockRequest: object = { environment: '', application: '', lockId: '' };

export const DeleteEnvironmentApplicationLockRequest = {
    encode(message: DeleteEnvironmentApplicationLockRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        if (message.application !== '') {
            writer.uint32(18).string(message.application);
        }
        if (message.lockId !== '') {
            writer.uint32(26).string(message.lockId);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): DeleteEnvironmentApplicationLockRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseDeleteEnvironmentApplicationLockRequest } as DeleteEnvironmentApplicationLockRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                case 2:
                    message.application = reader.string();
                    break;
                case 3:
                    message.lockId = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): DeleteEnvironmentApplicationLockRequest {
        const message = { ...baseDeleteEnvironmentApplicationLockRequest } as DeleteEnvironmentApplicationLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        if (object.application !== undefined && object.application !== null) {
            message.application = String(object.application);
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = String(object.lockId);
        }
        return message;
    },

    toJSON(message: DeleteEnvironmentApplicationLockRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        message.application !== undefined && (obj.application = message.application);
        message.lockId !== undefined && (obj.lockId = message.lockId);
        return obj;
    },

    fromPartial(object: DeepPartial<DeleteEnvironmentApplicationLockRequest>): DeleteEnvironmentApplicationLockRequest {
        const message = { ...baseDeleteEnvironmentApplicationLockRequest } as DeleteEnvironmentApplicationLockRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        if (object.application !== undefined && object.application !== null) {
            message.application = object.application;
        }
        if (object.lockId !== undefined && object.lockId !== null) {
            message.lockId = object.lockId;
        }
        return message;
    },
};

const baseDeployRequest: object = {
    environment: '',
    application: '',
    version: 0,
    ignoreAllLocks: false,
    lockBehavior: 0,
};

export const DeployRequest = {
    encode(message: DeployRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        if (message.application !== '') {
            writer.uint32(18).string(message.application);
        }
        if (message.version !== 0) {
            writer.uint32(24).uint64(message.version);
        }
        if (message.ignoreAllLocks === true) {
            writer.uint32(32).bool(message.ignoreAllLocks);
        }
        if (message.lockBehavior !== 0) {
            writer.uint32(40).int32(message.lockBehavior);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): DeployRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseDeployRequest } as DeployRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                case 2:
                    message.application = reader.string();
                    break;
                case 3:
                    message.version = longToNumber(reader.uint64() as Long);
                    break;
                case 4:
                    message.ignoreAllLocks = reader.bool();
                    break;
                case 5:
                    message.lockBehavior = reader.int32() as any;
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): DeployRequest {
        const message = { ...baseDeployRequest } as DeployRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        if (object.application !== undefined && object.application !== null) {
            message.application = String(object.application);
        }
        if (object.version !== undefined && object.version !== null) {
            message.version = Number(object.version);
        }
        if (object.ignoreAllLocks !== undefined && object.ignoreAllLocks !== null) {
            message.ignoreAllLocks = Boolean(object.ignoreAllLocks);
        }
        if (object.lockBehavior !== undefined && object.lockBehavior !== null) {
            message.lockBehavior = lockBehaviorFromJSON(object.lockBehavior);
        }
        return message;
    },

    toJSON(message: DeployRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        message.application !== undefined && (obj.application = message.application);
        message.version !== undefined && (obj.version = message.version);
        message.ignoreAllLocks !== undefined && (obj.ignoreAllLocks = message.ignoreAllLocks);
        message.lockBehavior !== undefined && (obj.lockBehavior = lockBehaviorToJSON(message.lockBehavior));
        return obj;
    },

    fromPartial(object: DeepPartial<DeployRequest>): DeployRequest {
        const message = { ...baseDeployRequest } as DeployRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        if (object.application !== undefined && object.application !== null) {
            message.application = object.application;
        }
        if (object.version !== undefined && object.version !== null) {
            message.version = object.version;
        }
        if (object.ignoreAllLocks !== undefined && object.ignoreAllLocks !== null) {
            message.ignoreAllLocks = object.ignoreAllLocks;
        }
        if (object.lockBehavior !== undefined && object.lockBehavior !== null) {
            message.lockBehavior = object.lockBehavior;
        }
        return message;
    },
};

const basePrepareUndeployRequest: object = { application: '' };

export const PrepareUndeployRequest = {
    encode(message: PrepareUndeployRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.application !== '') {
            writer.uint32(10).string(message.application);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): PrepareUndeployRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...basePrepareUndeployRequest } as PrepareUndeployRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.application = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): PrepareUndeployRequest {
        const message = { ...basePrepareUndeployRequest } as PrepareUndeployRequest;
        if (object.application !== undefined && object.application !== null) {
            message.application = String(object.application);
        }
        return message;
    },

    toJSON(message: PrepareUndeployRequest): unknown {
        const obj: any = {};
        message.application !== undefined && (obj.application = message.application);
        return obj;
    },

    fromPartial(object: DeepPartial<PrepareUndeployRequest>): PrepareUndeployRequest {
        const message = { ...basePrepareUndeployRequest } as PrepareUndeployRequest;
        if (object.application !== undefined && object.application !== null) {
            message.application = object.application;
        }
        return message;
    },
};

const baseReleaseTrainRequest: object = { environment: '' };

export const ReleaseTrainRequest = {
    encode(message: ReleaseTrainRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): ReleaseTrainRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseReleaseTrainRequest } as ReleaseTrainRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): ReleaseTrainRequest {
        const message = { ...baseReleaseTrainRequest } as ReleaseTrainRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        return message;
    },

    toJSON(message: ReleaseTrainRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        return obj;
    },

    fromPartial(object: DeepPartial<ReleaseTrainRequest>): ReleaseTrainRequest {
        const message = { ...baseReleaseTrainRequest } as ReleaseTrainRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        return message;
    },
};

const baseLock: object = { message: '' };

export const Lock = {
    encode(message: Lock, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.message !== '') {
            writer.uint32(10).string(message.message);
        }
        if (message.commit !== undefined) {
            Commit.encode(message.commit, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Lock {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseLock } as Lock;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.message = reader.string();
                    break;
                case 2:
                    message.commit = Commit.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Lock {
        const message = { ...baseLock } as Lock;
        if (object.message !== undefined && object.message !== null) {
            message.message = String(object.message);
        }
        if (object.commit !== undefined && object.commit !== null) {
            message.commit = Commit.fromJSON(object.commit);
        }
        return message;
    },

    toJSON(message: Lock): unknown {
        const obj: any = {};
        message.message !== undefined && (obj.message = message.message);
        message.commit !== undefined && (obj.commit = message.commit ? Commit.toJSON(message.commit) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<Lock>): Lock {
        const message = { ...baseLock } as Lock;
        if (object.message !== undefined && object.message !== null) {
            message.message = object.message;
        }
        if (object.commit !== undefined && object.commit !== null) {
            message.commit = Commit.fromPartial(object.commit);
        }
        return message;
    },
};

const baseLockedError: object = {};

export const LockedError = {
    encode(message: LockedError, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        Object.entries(message.environmentLocks).forEach(([key, value]) => {
            LockedError_EnvironmentLocksEntry.encode({ key: key as any, value }, writer.uint32(10).fork()).ldelim();
        });
        Object.entries(message.environmentApplicationLocks).forEach(([key, value]) => {
            LockedError_EnvironmentApplicationLocksEntry.encode(
                { key: key as any, value },
                writer.uint32(18).fork()
            ).ldelim();
        });
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): LockedError {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseLockedError } as LockedError;
        message.environmentLocks = {};
        message.environmentApplicationLocks = {};
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    const entry1 = LockedError_EnvironmentLocksEntry.decode(reader, reader.uint32());
                    if (entry1.value !== undefined) {
                        message.environmentLocks[entry1.key] = entry1.value;
                    }
                    break;
                case 2:
                    const entry2 = LockedError_EnvironmentApplicationLocksEntry.decode(reader, reader.uint32());
                    if (entry2.value !== undefined) {
                        message.environmentApplicationLocks[entry2.key] = entry2.value;
                    }
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): LockedError {
        const message = { ...baseLockedError } as LockedError;
        message.environmentLocks = {};
        message.environmentApplicationLocks = {};
        if (object.environmentLocks !== undefined && object.environmentLocks !== null) {
            Object.entries(object.environmentLocks).forEach(([key, value]) => {
                message.environmentLocks[key] = Lock.fromJSON(value);
            });
        }
        if (object.environmentApplicationLocks !== undefined && object.environmentApplicationLocks !== null) {
            Object.entries(object.environmentApplicationLocks).forEach(([key, value]) => {
                message.environmentApplicationLocks[key] = Lock.fromJSON(value);
            });
        }
        return message;
    },

    toJSON(message: LockedError): unknown {
        const obj: any = {};
        obj.environmentLocks = {};
        if (message.environmentLocks) {
            Object.entries(message.environmentLocks).forEach(([k, v]) => {
                obj.environmentLocks[k] = Lock.toJSON(v);
            });
        }
        obj.environmentApplicationLocks = {};
        if (message.environmentApplicationLocks) {
            Object.entries(message.environmentApplicationLocks).forEach(([k, v]) => {
                obj.environmentApplicationLocks[k] = Lock.toJSON(v);
            });
        }
        return obj;
    },

    fromPartial(object: DeepPartial<LockedError>): LockedError {
        const message = { ...baseLockedError } as LockedError;
        message.environmentLocks = {};
        message.environmentApplicationLocks = {};
        if (object.environmentLocks !== undefined && object.environmentLocks !== null) {
            Object.entries(object.environmentLocks).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.environmentLocks[key] = Lock.fromPartial(value);
                }
            });
        }
        if (object.environmentApplicationLocks !== undefined && object.environmentApplicationLocks !== null) {
            Object.entries(object.environmentApplicationLocks).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.environmentApplicationLocks[key] = Lock.fromPartial(value);
                }
            });
        }
        return message;
    },
};

const baseLockedError_EnvironmentLocksEntry: object = { key: '' };

export const LockedError_EnvironmentLocksEntry = {
    encode(message: LockedError_EnvironmentLocksEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Lock.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): LockedError_EnvironmentLocksEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseLockedError_EnvironmentLocksEntry } as LockedError_EnvironmentLocksEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Lock.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): LockedError_EnvironmentLocksEntry {
        const message = { ...baseLockedError_EnvironmentLocksEntry } as LockedError_EnvironmentLocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: LockedError_EnvironmentLocksEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined && (obj.value = message.value ? Lock.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<LockedError_EnvironmentLocksEntry>): LockedError_EnvironmentLocksEntry {
        const message = { ...baseLockedError_EnvironmentLocksEntry } as LockedError_EnvironmentLocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromPartial(object.value);
        }
        return message;
    },
};

const baseLockedError_EnvironmentApplicationLocksEntry: object = { key: '' };

export const LockedError_EnvironmentApplicationLocksEntry = {
    encode(
        message: LockedError_EnvironmentApplicationLocksEntry,
        writer: _m0.Writer = _m0.Writer.create()
    ): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Lock.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): LockedError_EnvironmentApplicationLocksEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = {
            ...baseLockedError_EnvironmentApplicationLocksEntry,
        } as LockedError_EnvironmentApplicationLocksEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Lock.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): LockedError_EnvironmentApplicationLocksEntry {
        const message = {
            ...baseLockedError_EnvironmentApplicationLocksEntry,
        } as LockedError_EnvironmentApplicationLocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: LockedError_EnvironmentApplicationLocksEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined && (obj.value = message.value ? Lock.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(
        object: DeepPartial<LockedError_EnvironmentApplicationLocksEntry>
    ): LockedError_EnvironmentApplicationLocksEntry {
        const message = {
            ...baseLockedError_EnvironmentApplicationLocksEntry,
        } as LockedError_EnvironmentApplicationLocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromPartial(object.value);
        }
        return message;
    },
};

const baseCreateEnvironmentRequest: object = { environment: '' };

export const CreateEnvironmentRequest = {
    encode(message: CreateEnvironmentRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.environment !== '') {
            writer.uint32(10).string(message.environment);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): CreateEnvironmentRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseCreateEnvironmentRequest } as CreateEnvironmentRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.environment = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): CreateEnvironmentRequest {
        const message = { ...baseCreateEnvironmentRequest } as CreateEnvironmentRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = String(object.environment);
        }
        return message;
    },

    toJSON(message: CreateEnvironmentRequest): unknown {
        const obj: any = {};
        message.environment !== undefined && (obj.environment = message.environment);
        return obj;
    },

    fromPartial(object: DeepPartial<CreateEnvironmentRequest>): CreateEnvironmentRequest {
        const message = { ...baseCreateEnvironmentRequest } as CreateEnvironmentRequest;
        if (object.environment !== undefined && object.environment !== null) {
            message.environment = object.environment;
        }
        return message;
    },
};

const baseGetOverviewRequest: object = {};

export const GetOverviewRequest = {
    encode(_: GetOverviewRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): GetOverviewRequest {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseGetOverviewRequest } as GetOverviewRequest;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(_: any): GetOverviewRequest {
        const message = { ...baseGetOverviewRequest } as GetOverviewRequest;
        return message;
    },

    toJSON(_: GetOverviewRequest): unknown {
        const obj: any = {};
        return obj;
    },

    fromPartial(_: DeepPartial<GetOverviewRequest>): GetOverviewRequest {
        const message = { ...baseGetOverviewRequest } as GetOverviewRequest;
        return message;
    },
};

const baseGetOverviewResponse: object = {};

export const GetOverviewResponse = {
    encode(message: GetOverviewResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        Object.entries(message.environments).forEach(([key, value]) => {
            GetOverviewResponse_EnvironmentsEntry.encode({ key: key as any, value }, writer.uint32(10).fork()).ldelim();
        });
        Object.entries(message.applications).forEach(([key, value]) => {
            GetOverviewResponse_ApplicationsEntry.encode({ key: key as any, value }, writer.uint32(18).fork()).ldelim();
        });
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): GetOverviewResponse {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseGetOverviewResponse } as GetOverviewResponse;
        message.environments = {};
        message.applications = {};
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    const entry1 = GetOverviewResponse_EnvironmentsEntry.decode(reader, reader.uint32());
                    if (entry1.value !== undefined) {
                        message.environments[entry1.key] = entry1.value;
                    }
                    break;
                case 2:
                    const entry2 = GetOverviewResponse_ApplicationsEntry.decode(reader, reader.uint32());
                    if (entry2.value !== undefined) {
                        message.applications[entry2.key] = entry2.value;
                    }
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): GetOverviewResponse {
        const message = { ...baseGetOverviewResponse } as GetOverviewResponse;
        message.environments = {};
        message.applications = {};
        if (object.environments !== undefined && object.environments !== null) {
            Object.entries(object.environments).forEach(([key, value]) => {
                message.environments[key] = Environment.fromJSON(value);
            });
        }
        if (object.applications !== undefined && object.applications !== null) {
            Object.entries(object.applications).forEach(([key, value]) => {
                message.applications[key] = Application.fromJSON(value);
            });
        }
        return message;
    },

    toJSON(message: GetOverviewResponse): unknown {
        const obj: any = {};
        obj.environments = {};
        if (message.environments) {
            Object.entries(message.environments).forEach(([k, v]) => {
                obj.environments[k] = Environment.toJSON(v);
            });
        }
        obj.applications = {};
        if (message.applications) {
            Object.entries(message.applications).forEach(([k, v]) => {
                obj.applications[k] = Application.toJSON(v);
            });
        }
        return obj;
    },

    fromPartial(object: DeepPartial<GetOverviewResponse>): GetOverviewResponse {
        const message = { ...baseGetOverviewResponse } as GetOverviewResponse;
        message.environments = {};
        message.applications = {};
        if (object.environments !== undefined && object.environments !== null) {
            Object.entries(object.environments).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.environments[key] = Environment.fromPartial(value);
                }
            });
        }
        if (object.applications !== undefined && object.applications !== null) {
            Object.entries(object.applications).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.applications[key] = Application.fromPartial(value);
                }
            });
        }
        return message;
    },
};

const baseGetOverviewResponse_EnvironmentsEntry: object = { key: '' };

export const GetOverviewResponse_EnvironmentsEntry = {
    encode(message: GetOverviewResponse_EnvironmentsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Environment.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): GetOverviewResponse_EnvironmentsEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseGetOverviewResponse_EnvironmentsEntry } as GetOverviewResponse_EnvironmentsEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Environment.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): GetOverviewResponse_EnvironmentsEntry {
        const message = { ...baseGetOverviewResponse_EnvironmentsEntry } as GetOverviewResponse_EnvironmentsEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Environment.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: GetOverviewResponse_EnvironmentsEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined && (obj.value = message.value ? Environment.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<GetOverviewResponse_EnvironmentsEntry>): GetOverviewResponse_EnvironmentsEntry {
        const message = { ...baseGetOverviewResponse_EnvironmentsEntry } as GetOverviewResponse_EnvironmentsEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Environment.fromPartial(object.value);
        }
        return message;
    },
};

const baseGetOverviewResponse_ApplicationsEntry: object = { key: '' };

export const GetOverviewResponse_ApplicationsEntry = {
    encode(message: GetOverviewResponse_ApplicationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Application.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): GetOverviewResponse_ApplicationsEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseGetOverviewResponse_ApplicationsEntry } as GetOverviewResponse_ApplicationsEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Application.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): GetOverviewResponse_ApplicationsEntry {
        const message = { ...baseGetOverviewResponse_ApplicationsEntry } as GetOverviewResponse_ApplicationsEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Application.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: GetOverviewResponse_ApplicationsEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined && (obj.value = message.value ? Application.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<GetOverviewResponse_ApplicationsEntry>): GetOverviewResponse_ApplicationsEntry {
        const message = { ...baseGetOverviewResponse_ApplicationsEntry } as GetOverviewResponse_ApplicationsEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Application.fromPartial(object.value);
        }
        return message;
    },
};

const baseEnvironment: object = { name: '' };

export const Environment = {
    encode(message: Environment, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.name !== '') {
            writer.uint32(10).string(message.name);
        }
        if (message.config !== undefined) {
            Environment_Config.encode(message.config, writer.uint32(18).fork()).ldelim();
        }
        Object.entries(message.locks).forEach(([key, value]) => {
            Environment_LocksEntry.encode({ key: key as any, value }, writer.uint32(26).fork()).ldelim();
        });
        Object.entries(message.applications).forEach(([key, value]) => {
            Environment_ApplicationsEntry.encode({ key: key as any, value }, writer.uint32(34).fork()).ldelim();
        });
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment } as Environment;
        message.locks = {};
        message.applications = {};
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.name = reader.string();
                    break;
                case 2:
                    message.config = Environment_Config.decode(reader, reader.uint32());
                    break;
                case 3:
                    const entry3 = Environment_LocksEntry.decode(reader, reader.uint32());
                    if (entry3.value !== undefined) {
                        message.locks[entry3.key] = entry3.value;
                    }
                    break;
                case 4:
                    const entry4 = Environment_ApplicationsEntry.decode(reader, reader.uint32());
                    if (entry4.value !== undefined) {
                        message.applications[entry4.key] = entry4.value;
                    }
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment {
        const message = { ...baseEnvironment } as Environment;
        message.locks = {};
        message.applications = {};
        if (object.name !== undefined && object.name !== null) {
            message.name = String(object.name);
        }
        if (object.config !== undefined && object.config !== null) {
            message.config = Environment_Config.fromJSON(object.config);
        }
        if (object.locks !== undefined && object.locks !== null) {
            Object.entries(object.locks).forEach(([key, value]) => {
                message.locks[key] = Lock.fromJSON(value);
            });
        }
        if (object.applications !== undefined && object.applications !== null) {
            Object.entries(object.applications).forEach(([key, value]) => {
                message.applications[key] = Environment_Application.fromJSON(value);
            });
        }
        return message;
    },

    toJSON(message: Environment): unknown {
        const obj: any = {};
        message.name !== undefined && (obj.name = message.name);
        message.config !== undefined &&
            (obj.config = message.config ? Environment_Config.toJSON(message.config) : undefined);
        obj.locks = {};
        if (message.locks) {
            Object.entries(message.locks).forEach(([k, v]) => {
                obj.locks[k] = Lock.toJSON(v);
            });
        }
        obj.applications = {};
        if (message.applications) {
            Object.entries(message.applications).forEach(([k, v]) => {
                obj.applications[k] = Environment_Application.toJSON(v);
            });
        }
        return obj;
    },

    fromPartial(object: DeepPartial<Environment>): Environment {
        const message = { ...baseEnvironment } as Environment;
        message.locks = {};
        message.applications = {};
        if (object.name !== undefined && object.name !== null) {
            message.name = object.name;
        }
        if (object.config !== undefined && object.config !== null) {
            message.config = Environment_Config.fromPartial(object.config);
        }
        if (object.locks !== undefined && object.locks !== null) {
            Object.entries(object.locks).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.locks[key] = Lock.fromPartial(value);
                }
            });
        }
        if (object.applications !== undefined && object.applications !== null) {
            Object.entries(object.applications).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.applications[key] = Environment_Application.fromPartial(value);
                }
            });
        }
        return message;
    },
};

const baseEnvironment_Config: object = {};

export const Environment_Config = {
    encode(message: Environment_Config, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.upstream !== undefined) {
            Environment_Config_Upstream.encode(message.upstream, writer.uint32(10).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment_Config {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment_Config } as Environment_Config;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.upstream = Environment_Config_Upstream.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment_Config {
        const message = { ...baseEnvironment_Config } as Environment_Config;
        if (object.upstream !== undefined && object.upstream !== null) {
            message.upstream = Environment_Config_Upstream.fromJSON(object.upstream);
        }
        return message;
    },

    toJSON(message: Environment_Config): unknown {
        const obj: any = {};
        message.upstream !== undefined &&
            (obj.upstream = message.upstream ? Environment_Config_Upstream.toJSON(message.upstream) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<Environment_Config>): Environment_Config {
        const message = { ...baseEnvironment_Config } as Environment_Config;
        if (object.upstream !== undefined && object.upstream !== null) {
            message.upstream = Environment_Config_Upstream.fromPartial(object.upstream);
        }
        return message;
    },
};

const baseEnvironment_Config_Upstream: object = {};

export const Environment_Config_Upstream = {
    encode(message: Environment_Config_Upstream, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.upstream?.$case === 'environment') {
            writer.uint32(10).string(message.upstream.environment);
        }
        if (message.upstream?.$case === 'latest') {
            writer.uint32(16).bool(message.upstream.latest);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment_Config_Upstream {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment_Config_Upstream } as Environment_Config_Upstream;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.upstream = { $case: 'environment', environment: reader.string() };
                    break;
                case 2:
                    message.upstream = { $case: 'latest', latest: reader.bool() };
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment_Config_Upstream {
        const message = { ...baseEnvironment_Config_Upstream } as Environment_Config_Upstream;
        if (object.environment !== undefined && object.environment !== null) {
            message.upstream = { $case: 'environment', environment: String(object.environment) };
        }
        if (object.latest !== undefined && object.latest !== null) {
            message.upstream = { $case: 'latest', latest: Boolean(object.latest) };
        }
        return message;
    },

    toJSON(message: Environment_Config_Upstream): unknown {
        const obj: any = {};
        message.upstream?.$case === 'environment' && (obj.environment = message.upstream?.environment);
        message.upstream?.$case === 'latest' && (obj.latest = message.upstream?.latest);
        return obj;
    },

    fromPartial(object: DeepPartial<Environment_Config_Upstream>): Environment_Config_Upstream {
        const message = { ...baseEnvironment_Config_Upstream } as Environment_Config_Upstream;
        if (
            object.upstream?.$case === 'environment' &&
            object.upstream?.environment !== undefined &&
            object.upstream?.environment !== null
        ) {
            message.upstream = { $case: 'environment', environment: object.upstream.environment };
        }
        if (
            object.upstream?.$case === 'latest' &&
            object.upstream?.latest !== undefined &&
            object.upstream?.latest !== null
        ) {
            message.upstream = { $case: 'latest', latest: object.upstream.latest };
        }
        return message;
    },
};

const baseEnvironment_Application: object = { name: '', version: 0, queuedVersion: 0, undeployVersion: false };

export const Environment_Application = {
    encode(message: Environment_Application, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.name !== '') {
            writer.uint32(10).string(message.name);
        }
        if (message.version !== 0) {
            writer.uint32(16).uint64(message.version);
        }
        Object.entries(message.locks).forEach(([key, value]) => {
            Environment_Application_LocksEntry.encode({ key: key as any, value }, writer.uint32(26).fork()).ldelim();
        });
        if (message.queuedVersion !== 0) {
            writer.uint32(32).uint64(message.queuedVersion);
        }
        if (message.versionCommit !== undefined) {
            Commit.encode(message.versionCommit, writer.uint32(42).fork()).ldelim();
        }
        if (message.undeployVersion === true) {
            writer.uint32(48).bool(message.undeployVersion);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment_Application {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment_Application } as Environment_Application;
        message.locks = {};
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.name = reader.string();
                    break;
                case 2:
                    message.version = longToNumber(reader.uint64() as Long);
                    break;
                case 3:
                    const entry3 = Environment_Application_LocksEntry.decode(reader, reader.uint32());
                    if (entry3.value !== undefined) {
                        message.locks[entry3.key] = entry3.value;
                    }
                    break;
                case 4:
                    message.queuedVersion = longToNumber(reader.uint64() as Long);
                    break;
                case 5:
                    message.versionCommit = Commit.decode(reader, reader.uint32());
                    break;
                case 6:
                    message.undeployVersion = reader.bool();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment_Application {
        const message = { ...baseEnvironment_Application } as Environment_Application;
        message.locks = {};
        if (object.name !== undefined && object.name !== null) {
            message.name = String(object.name);
        }
        if (object.version !== undefined && object.version !== null) {
            message.version = Number(object.version);
        }
        if (object.locks !== undefined && object.locks !== null) {
            Object.entries(object.locks).forEach(([key, value]) => {
                message.locks[key] = Lock.fromJSON(value);
            });
        }
        if (object.queuedVersion !== undefined && object.queuedVersion !== null) {
            message.queuedVersion = Number(object.queuedVersion);
        }
        if (object.versionCommit !== undefined && object.versionCommit !== null) {
            message.versionCommit = Commit.fromJSON(object.versionCommit);
        }
        if (object.undeployVersion !== undefined && object.undeployVersion !== null) {
            message.undeployVersion = Boolean(object.undeployVersion);
        }
        return message;
    },

    toJSON(message: Environment_Application): unknown {
        const obj: any = {};
        message.name !== undefined && (obj.name = message.name);
        message.version !== undefined && (obj.version = message.version);
        obj.locks = {};
        if (message.locks) {
            Object.entries(message.locks).forEach(([k, v]) => {
                obj.locks[k] = Lock.toJSON(v);
            });
        }
        message.queuedVersion !== undefined && (obj.queuedVersion = message.queuedVersion);
        message.versionCommit !== undefined &&
            (obj.versionCommit = message.versionCommit ? Commit.toJSON(message.versionCommit) : undefined);
        message.undeployVersion !== undefined && (obj.undeployVersion = message.undeployVersion);
        return obj;
    },

    fromPartial(object: DeepPartial<Environment_Application>): Environment_Application {
        const message = { ...baseEnvironment_Application } as Environment_Application;
        message.locks = {};
        if (object.name !== undefined && object.name !== null) {
            message.name = object.name;
        }
        if (object.version !== undefined && object.version !== null) {
            message.version = object.version;
        }
        if (object.locks !== undefined && object.locks !== null) {
            Object.entries(object.locks).forEach(([key, value]) => {
                if (value !== undefined) {
                    message.locks[key] = Lock.fromPartial(value);
                }
            });
        }
        if (object.queuedVersion !== undefined && object.queuedVersion !== null) {
            message.queuedVersion = object.queuedVersion;
        }
        if (object.versionCommit !== undefined && object.versionCommit !== null) {
            message.versionCommit = Commit.fromPartial(object.versionCommit);
        }
        if (object.undeployVersion !== undefined && object.undeployVersion !== null) {
            message.undeployVersion = object.undeployVersion;
        }
        return message;
    },
};

const baseEnvironment_Application_LocksEntry: object = { key: '' };

export const Environment_Application_LocksEntry = {
    encode(message: Environment_Application_LocksEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Lock.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment_Application_LocksEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment_Application_LocksEntry } as Environment_Application_LocksEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Lock.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment_Application_LocksEntry {
        const message = { ...baseEnvironment_Application_LocksEntry } as Environment_Application_LocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: Environment_Application_LocksEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined && (obj.value = message.value ? Lock.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<Environment_Application_LocksEntry>): Environment_Application_LocksEntry {
        const message = { ...baseEnvironment_Application_LocksEntry } as Environment_Application_LocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromPartial(object.value);
        }
        return message;
    },
};

const baseEnvironment_LocksEntry: object = { key: '' };

export const Environment_LocksEntry = {
    encode(message: Environment_LocksEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Lock.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment_LocksEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment_LocksEntry } as Environment_LocksEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Lock.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment_LocksEntry {
        const message = { ...baseEnvironment_LocksEntry } as Environment_LocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: Environment_LocksEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined && (obj.value = message.value ? Lock.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<Environment_LocksEntry>): Environment_LocksEntry {
        const message = { ...baseEnvironment_LocksEntry } as Environment_LocksEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Lock.fromPartial(object.value);
        }
        return message;
    },
};

const baseEnvironment_ApplicationsEntry: object = { key: '' };

export const Environment_ApplicationsEntry = {
    encode(message: Environment_ApplicationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.key !== '') {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== undefined) {
            Environment_Application.encode(message.value, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Environment_ApplicationsEntry {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEnvironment_ApplicationsEntry } as Environment_ApplicationsEntry;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.key = reader.string();
                    break;
                case 2:
                    message.value = Environment_Application.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Environment_ApplicationsEntry {
        const message = { ...baseEnvironment_ApplicationsEntry } as Environment_ApplicationsEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Environment_Application.fromJSON(object.value);
        }
        return message;
    },

    toJSON(message: Environment_ApplicationsEntry): unknown {
        const obj: any = {};
        message.key !== undefined && (obj.key = message.key);
        message.value !== undefined &&
            (obj.value = message.value ? Environment_Application.toJSON(message.value) : undefined);
        return obj;
    },

    fromPartial(object: DeepPartial<Environment_ApplicationsEntry>): Environment_ApplicationsEntry {
        const message = { ...baseEnvironment_ApplicationsEntry } as Environment_ApplicationsEntry;
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = Environment_Application.fromPartial(object.value);
        }
        return message;
    },
};

const baseRelease: object = {
    version: 0,
    sourceCommitId: '',
    sourceAuthor: '',
    sourceMessage: '',
    undeployVersion: false,
};

export const Release = {
    encode(message: Release, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.version !== 0) {
            writer.uint32(8).uint64(message.version);
        }
        if (message.sourceCommitId !== '') {
            writer.uint32(18).string(message.sourceCommitId);
        }
        if (message.sourceAuthor !== '') {
            writer.uint32(26).string(message.sourceAuthor);
        }
        if (message.sourceMessage !== '') {
            writer.uint32(34).string(message.sourceMessage);
        }
        if (message.commit !== undefined) {
            Commit.encode(message.commit, writer.uint32(42).fork()).ldelim();
        }
        if (message.undeployVersion === true) {
            writer.uint32(48).bool(message.undeployVersion);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Release {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseRelease } as Release;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.version = longToNumber(reader.uint64() as Long);
                    break;
                case 2:
                    message.sourceCommitId = reader.string();
                    break;
                case 3:
                    message.sourceAuthor = reader.string();
                    break;
                case 4:
                    message.sourceMessage = reader.string();
                    break;
                case 5:
                    message.commit = Commit.decode(reader, reader.uint32());
                    break;
                case 6:
                    message.undeployVersion = reader.bool();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Release {
        const message = { ...baseRelease } as Release;
        if (object.version !== undefined && object.version !== null) {
            message.version = Number(object.version);
        }
        if (object.sourceCommitId !== undefined && object.sourceCommitId !== null) {
            message.sourceCommitId = String(object.sourceCommitId);
        }
        if (object.sourceAuthor !== undefined && object.sourceAuthor !== null) {
            message.sourceAuthor = String(object.sourceAuthor);
        }
        if (object.sourceMessage !== undefined && object.sourceMessage !== null) {
            message.sourceMessage = String(object.sourceMessage);
        }
        if (object.commit !== undefined && object.commit !== null) {
            message.commit = Commit.fromJSON(object.commit);
        }
        if (object.undeployVersion !== undefined && object.undeployVersion !== null) {
            message.undeployVersion = Boolean(object.undeployVersion);
        }
        return message;
    },

    toJSON(message: Release): unknown {
        const obj: any = {};
        message.version !== undefined && (obj.version = message.version);
        message.sourceCommitId !== undefined && (obj.sourceCommitId = message.sourceCommitId);
        message.sourceAuthor !== undefined && (obj.sourceAuthor = message.sourceAuthor);
        message.sourceMessage !== undefined && (obj.sourceMessage = message.sourceMessage);
        message.commit !== undefined && (obj.commit = message.commit ? Commit.toJSON(message.commit) : undefined);
        message.undeployVersion !== undefined && (obj.undeployVersion = message.undeployVersion);
        return obj;
    },

    fromPartial(object: DeepPartial<Release>): Release {
        const message = { ...baseRelease } as Release;
        if (object.version !== undefined && object.version !== null) {
            message.version = object.version;
        }
        if (object.sourceCommitId !== undefined && object.sourceCommitId !== null) {
            message.sourceCommitId = object.sourceCommitId;
        }
        if (object.sourceAuthor !== undefined && object.sourceAuthor !== null) {
            message.sourceAuthor = object.sourceAuthor;
        }
        if (object.sourceMessage !== undefined && object.sourceMessage !== null) {
            message.sourceMessage = object.sourceMessage;
        }
        if (object.commit !== undefined && object.commit !== null) {
            message.commit = Commit.fromPartial(object.commit);
        }
        if (object.undeployVersion !== undefined && object.undeployVersion !== null) {
            message.undeployVersion = object.undeployVersion;
        }
        return message;
    },
};

const baseApplication: object = { name: '' };

export const Application = {
    encode(message: Application, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.name !== '') {
            writer.uint32(10).string(message.name);
        }
        for (const v of message.releases) {
            Release.encode(v!, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Application {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseApplication } as Application;
        message.releases = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.name = reader.string();
                    break;
                case 2:
                    message.releases.push(Release.decode(reader, reader.uint32()));
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Application {
        const message = { ...baseApplication } as Application;
        message.releases = [];
        if (object.name !== undefined && object.name !== null) {
            message.name = String(object.name);
        }
        if (object.releases !== undefined && object.releases !== null) {
            for (const e of object.releases) {
                message.releases.push(Release.fromJSON(e));
            }
        }
        return message;
    },

    toJSON(message: Application): unknown {
        const obj: any = {};
        message.name !== undefined && (obj.name = message.name);
        if (message.releases) {
            obj.releases = message.releases.map((e) => (e ? Release.toJSON(e) : undefined));
        } else {
            obj.releases = [];
        }
        return obj;
    },

    fromPartial(object: DeepPartial<Application>): Application {
        const message = { ...baseApplication } as Application;
        message.releases = [];
        if (object.name !== undefined && object.name !== null) {
            message.name = object.name;
        }
        if (object.releases !== undefined && object.releases !== null) {
            for (const e of object.releases) {
                message.releases.push(Release.fromPartial(e));
            }
        }
        return message;
    },
};

const baseCommit: object = { authorName: '', authorEmail: '' };

export const Commit = {
    encode(message: Commit, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
        if (message.authorTime !== undefined) {
            Timestamp.encode(toTimestamp(message.authorTime), writer.uint32(10).fork()).ldelim();
        }
        if (message.authorName !== '') {
            writer.uint32(18).string(message.authorName);
        }
        if (message.authorEmail !== '') {
            writer.uint32(26).string(message.authorEmail);
        }
        return writer;
    },

    decode(input: _m0.Reader | Uint8Array, length?: number): Commit {
        const reader = input instanceof _m0.Reader ? input : new _m0.Reader(input);
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseCommit } as Commit;
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.authorTime = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
                    break;
                case 2:
                    message.authorName = reader.string();
                    break;
                case 3:
                    message.authorEmail = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },

    fromJSON(object: any): Commit {
        const message = { ...baseCommit } as Commit;
        if (object.authorTime !== undefined && object.authorTime !== null) {
            message.authorTime = fromJsonTimestamp(object.authorTime);
        }
        if (object.authorName !== undefined && object.authorName !== null) {
            message.authorName = String(object.authorName);
        }
        if (object.authorEmail !== undefined && object.authorEmail !== null) {
            message.authorEmail = String(object.authorEmail);
        }
        return message;
    },

    toJSON(message: Commit): unknown {
        const obj: any = {};
        message.authorTime !== undefined && (obj.authorTime = message.authorTime.toISOString());
        message.authorName !== undefined && (obj.authorName = message.authorName);
        message.authorEmail !== undefined && (obj.authorEmail = message.authorEmail);
        return obj;
    },

    fromPartial(object: DeepPartial<Commit>): Commit {
        const message = { ...baseCommit } as Commit;
        if (object.authorTime !== undefined && object.authorTime !== null) {
            message.authorTime = object.authorTime;
        }
        if (object.authorName !== undefined && object.authorName !== null) {
            message.authorName = object.authorName;
        }
        if (object.authorEmail !== undefined && object.authorEmail !== null) {
            message.authorEmail = object.authorEmail;
        }
        return message;
    },
};

export interface LockService {
    CreateEnvironmentLock(request: DeepPartial<CreateEnvironmentLockRequest>, metadata?: grpc.Metadata): Promise<Empty>;
    DeleteEnvironmentLock(request: DeepPartial<DeleteEnvironmentLockRequest>, metadata?: grpc.Metadata): Promise<Empty>;
    CreateEnvironmentApplicationLock(
        request: DeepPartial<CreateEnvironmentApplicationLockRequest>,
        metadata?: grpc.Metadata
    ): Promise<Empty>;
    DeleteEnvironmentApplicationLock(
        request: DeepPartial<DeleteEnvironmentApplicationLockRequest>,
        metadata?: grpc.Metadata
    ): Promise<Empty>;
}

export class LockServiceClientImpl implements LockService {
    private readonly rpc: Rpc;

    constructor(rpc: Rpc) {
        this.rpc = rpc;
        this.CreateEnvironmentLock = this.CreateEnvironmentLock.bind(this);
        this.DeleteEnvironmentLock = this.DeleteEnvironmentLock.bind(this);
        this.CreateEnvironmentApplicationLock = this.CreateEnvironmentApplicationLock.bind(this);
        this.DeleteEnvironmentApplicationLock = this.DeleteEnvironmentApplicationLock.bind(this);
    }

    CreateEnvironmentLock(
        request: DeepPartial<CreateEnvironmentLockRequest>,
        metadata?: grpc.Metadata
    ): Promise<Empty> {
        return this.rpc.unary(
            LockServiceCreateEnvironmentLockDesc,
            CreateEnvironmentLockRequest.fromPartial(request),
            metadata
        );
    }

    DeleteEnvironmentLock(
        request: DeepPartial<DeleteEnvironmentLockRequest>,
        metadata?: grpc.Metadata
    ): Promise<Empty> {
        return this.rpc.unary(
            LockServiceDeleteEnvironmentLockDesc,
            DeleteEnvironmentLockRequest.fromPartial(request),
            metadata
        );
    }

    CreateEnvironmentApplicationLock(
        request: DeepPartial<CreateEnvironmentApplicationLockRequest>,
        metadata?: grpc.Metadata
    ): Promise<Empty> {
        return this.rpc.unary(
            LockServiceCreateEnvironmentApplicationLockDesc,
            CreateEnvironmentApplicationLockRequest.fromPartial(request),
            metadata
        );
    }

    DeleteEnvironmentApplicationLock(
        request: DeepPartial<DeleteEnvironmentApplicationLockRequest>,
        metadata?: grpc.Metadata
    ): Promise<Empty> {
        return this.rpc.unary(
            LockServiceDeleteEnvironmentApplicationLockDesc,
            DeleteEnvironmentApplicationLockRequest.fromPartial(request),
            metadata
        );
    }
}

export const LockServiceDesc = {
    serviceName: 'api.v1.LockService',
};

export const LockServiceCreateEnvironmentLockDesc: UnaryMethodDefinitionish = {
    methodName: 'CreateEnvironmentLock',
    service: LockServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return CreateEnvironmentLockRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export const LockServiceDeleteEnvironmentLockDesc: UnaryMethodDefinitionish = {
    methodName: 'DeleteEnvironmentLock',
    service: LockServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return DeleteEnvironmentLockRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export const LockServiceCreateEnvironmentApplicationLockDesc: UnaryMethodDefinitionish = {
    methodName: 'CreateEnvironmentApplicationLock',
    service: LockServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return CreateEnvironmentApplicationLockRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export const LockServiceDeleteEnvironmentApplicationLockDesc: UnaryMethodDefinitionish = {
    methodName: 'DeleteEnvironmentApplicationLock',
    service: LockServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return DeleteEnvironmentApplicationLockRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export interface BatchService {
    ProcessBatch(request: DeepPartial<BatchRequest>, metadata?: grpc.Metadata): Promise<Empty>;
}

export class BatchServiceClientImpl implements BatchService {
    private readonly rpc: Rpc;

    constructor(rpc: Rpc) {
        this.rpc = rpc;
        this.ProcessBatch = this.ProcessBatch.bind(this);
    }

    ProcessBatch(request: DeepPartial<BatchRequest>, metadata?: grpc.Metadata): Promise<Empty> {
        return this.rpc.unary(BatchServiceProcessBatchDesc, BatchRequest.fromPartial(request), metadata);
    }
}

export const BatchServiceDesc = {
    serviceName: 'api.v1.BatchService',
};

export const BatchServiceProcessBatchDesc: UnaryMethodDefinitionish = {
    methodName: 'ProcessBatch',
    service: BatchServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return BatchRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export interface DeployService {
    Deploy(request: DeepPartial<DeployRequest>, metadata?: grpc.Metadata): Promise<Empty>;
    ReleaseTrain(request: DeepPartial<ReleaseTrainRequest>, metadata?: grpc.Metadata): Promise<Empty>;
}

export class DeployServiceClientImpl implements DeployService {
    private readonly rpc: Rpc;

    constructor(rpc: Rpc) {
        this.rpc = rpc;
        this.Deploy = this.Deploy.bind(this);
        this.ReleaseTrain = this.ReleaseTrain.bind(this);
    }

    Deploy(request: DeepPartial<DeployRequest>, metadata?: grpc.Metadata): Promise<Empty> {
        return this.rpc.unary(DeployServiceDeployDesc, DeployRequest.fromPartial(request), metadata);
    }

    ReleaseTrain(request: DeepPartial<ReleaseTrainRequest>, metadata?: grpc.Metadata): Promise<Empty> {
        return this.rpc.unary(DeployServiceReleaseTrainDesc, ReleaseTrainRequest.fromPartial(request), metadata);
    }
}

export const DeployServiceDesc = {
    serviceName: 'api.v1.DeployService',
};

export const DeployServiceDeployDesc: UnaryMethodDefinitionish = {
    methodName: 'Deploy',
    service: DeployServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return DeployRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export const DeployServiceReleaseTrainDesc: UnaryMethodDefinitionish = {
    methodName: 'ReleaseTrain',
    service: DeployServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return ReleaseTrainRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export interface EnvironmentService {
    CreateEnvironment(request: DeepPartial<CreateEnvironmentRequest>, metadata?: grpc.Metadata): Promise<Empty>;
}

export class EnvironmentServiceClientImpl implements EnvironmentService {
    private readonly rpc: Rpc;

    constructor(rpc: Rpc) {
        this.rpc = rpc;
        this.CreateEnvironment = this.CreateEnvironment.bind(this);
    }

    CreateEnvironment(request: DeepPartial<CreateEnvironmentRequest>, metadata?: grpc.Metadata): Promise<Empty> {
        return this.rpc.unary(
            EnvironmentServiceCreateEnvironmentDesc,
            CreateEnvironmentRequest.fromPartial(request),
            metadata
        );
    }
}

export const EnvironmentServiceDesc = {
    serviceName: 'api.v1.EnvironmentService',
};

export const EnvironmentServiceCreateEnvironmentDesc: UnaryMethodDefinitionish = {
    methodName: 'CreateEnvironment',
    service: EnvironmentServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return CreateEnvironmentRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...Empty.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export interface OverviewService {
    GetOverview(request: DeepPartial<GetOverviewRequest>, metadata?: grpc.Metadata): Promise<GetOverviewResponse>;
    StreamOverview(request: DeepPartial<GetOverviewRequest>, metadata?: grpc.Metadata): Observable<GetOverviewResponse>;
}

export class OverviewServiceClientImpl implements OverviewService {
    private readonly rpc: Rpc;

    constructor(rpc: Rpc) {
        this.rpc = rpc;
        this.GetOverview = this.GetOverview.bind(this);
        this.StreamOverview = this.StreamOverview.bind(this);
    }

    GetOverview(request: DeepPartial<GetOverviewRequest>, metadata?: grpc.Metadata): Promise<GetOverviewResponse> {
        return this.rpc.unary(OverviewServiceGetOverviewDesc, GetOverviewRequest.fromPartial(request), metadata);
    }

    StreamOverview(
        request: DeepPartial<GetOverviewRequest>,
        metadata?: grpc.Metadata
    ): Observable<GetOverviewResponse> {
        return this.rpc.invoke(OverviewServiceStreamOverviewDesc, GetOverviewRequest.fromPartial(request), metadata);
    }
}

export const OverviewServiceDesc = {
    serviceName: 'api.v1.OverviewService',
};

export const OverviewServiceGetOverviewDesc: UnaryMethodDefinitionish = {
    methodName: 'GetOverview',
    service: OverviewServiceDesc,
    requestStream: false,
    responseStream: false,
    requestType: {
        serializeBinary() {
            return GetOverviewRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...GetOverviewResponse.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

export const OverviewServiceStreamOverviewDesc: UnaryMethodDefinitionish = {
    methodName: 'StreamOverview',
    service: OverviewServiceDesc,
    requestStream: false,
    responseStream: true,
    requestType: {
        serializeBinary() {
            return GetOverviewRequest.encode(this).finish();
        },
    } as any,
    responseType: {
        deserializeBinary(data: Uint8Array) {
            return {
                ...GetOverviewResponse.decode(data),
                toObject() {
                    return this;
                },
            };
        },
    } as any,
};

interface UnaryMethodDefinitionishR extends grpc.UnaryMethodDefinition<any, any> {
    requestStream: any;
    responseStream: any;
}

type UnaryMethodDefinitionish = UnaryMethodDefinitionishR;

interface Rpc {
    unary<T extends UnaryMethodDefinitionish>(
        methodDesc: T,
        request: any,
        metadata: grpc.Metadata | undefined
    ): Promise<any>;
    invoke<T extends UnaryMethodDefinitionish>(
        methodDesc: T,
        request: any,
        metadata: grpc.Metadata | undefined
    ): Observable<any>;
}

export class GrpcWebImpl {
    private host: string;
    private options: {
        transport?: grpc.TransportFactory;
        streamingTransport?: grpc.TransportFactory;
        debug?: boolean;
        metadata?: grpc.Metadata;
    };

    constructor(
        host: string,
        options: {
            transport?: grpc.TransportFactory;
            streamingTransport?: grpc.TransportFactory;
            debug?: boolean;
            metadata?: grpc.Metadata;
        }
    ) {
        this.host = host;
        this.options = options;
    }

    unary<T extends UnaryMethodDefinitionish>(
        methodDesc: T,
        _request: any,
        metadata: grpc.Metadata | undefined
    ): Promise<any> {
        const request = { ..._request, ...methodDesc.requestType };
        const maybeCombinedMetadata =
            metadata && this.options.metadata
                ? new BrowserHeaders({ ...this.options?.metadata.headersMap, ...metadata?.headersMap })
                : metadata || this.options.metadata;
        return new Promise((resolve, reject) => {
            grpc.unary(methodDesc, {
                request,
                host: this.host,
                metadata: maybeCombinedMetadata,
                transport: this.options.transport,
                debug: this.options.debug,
                onEnd: function (response) {
                    if (response.status === grpc.Code.OK) {
                        resolve(response.message);
                    } else {
                        const err = new Error(response.statusMessage) as any;
                        err.code = response.status;
                        err.metadata = response.trailers;
                        reject(err);
                    }
                },
            });
        });
    }

    invoke<T extends UnaryMethodDefinitionish>(
        methodDesc: T,
        _request: any,
        metadata: grpc.Metadata | undefined
    ): Observable<any> {
        // Status Response Codes (https://developers.google.com/maps-booking/reference/grpc-api/status_codes)
        const upStreamCodes = [2, 4, 8, 9, 10, 13, 14, 15];
        const DEFAULT_TIMEOUT_TIME: number = 3_000;
        const request = { ..._request, ...methodDesc.requestType };
        const maybeCombinedMetadata =
            metadata && this.options.metadata
                ? new BrowserHeaders({ ...this.options?.metadata.headersMap, ...metadata?.headersMap })
                : metadata || this.options.metadata;
        return new Observable((observer) => {
            const upStream = () => {
                const client = grpc.invoke(methodDesc, {
                    host: this.host,
                    request,
                    transport: this.options.streamingTransport || this.options.transport,
                    metadata: maybeCombinedMetadata,
                    debug: this.options.debug,
                    onMessage: (next) => observer.next(next),
                    onEnd: (code: grpc.Code, message: string) => {
                        if (code === 0) {
                            observer.complete();
                        } else if (upStreamCodes.includes(code)) {
                            setTimeout(upStream, DEFAULT_TIMEOUT_TIME);
                        } else {
                            observer.error(new Error(`Error ${code} ${message}`));
                        }
                    },
                });
                observer.add(() => client.close());
            };
            upStream();
        }).pipe(share());
    }
}

declare var self: any | undefined;
declare var window: any | undefined;
var globalThis: any = (() => {
    if (typeof globalThis !== 'undefined') return globalThis;
    if (typeof self !== 'undefined') return self;
    if (typeof window !== 'undefined') return window;
    if (typeof global !== 'undefined') return global;
    throw 'Unable to locate global object';
})();

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;
export type DeepPartial<T> = T extends Builtin
    ? T
    : T extends Array<infer U>
    ? Array<DeepPartial<U>>
    : T extends ReadonlyArray<infer U>
    ? ReadonlyArray<DeepPartial<U>>
    : T extends { $case: string }
    ? { [K in keyof Omit<T, '$case'>]?: DeepPartial<T[K]> } & { $case: T['$case'] }
    : T extends {}
    ? { [K in keyof T]?: DeepPartial<T[K]> }
    : Partial<T>;

function toTimestamp(date: Date): Timestamp {
    const seconds = date.getTime() / 1_000;
    const nanos = (date.getTime() % 1_000) * 1_000_000;
    return { seconds, nanos };
}

function fromTimestamp(t: Timestamp): Date {
    let millis = t.seconds * 1_000;
    millis += t.nanos / 1_000_000;
    return new Date(millis);
}

function fromJsonTimestamp(o: any): Date {
    if (o instanceof Date) {
        return o;
    } else if (typeof o === 'string') {
        return new Date(o);
    } else {
        return fromTimestamp(Timestamp.fromJSON(o));
    }
}

function longToNumber(long: Long): number {
    if (long.gt(Number.MAX_SAFE_INTEGER)) {
        throw new globalThis.Error('Value is larger than Number.MAX_SAFE_INTEGER');
    }
    return long.toNumber();
}

if (_m0.util.Long !== Long) {
    _m0.util.Long = Long as any;
    _m0.configure();
}
