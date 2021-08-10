/* Autogenerated file. Do not edit manually. */
/* tslint:disable */
/* eslint-disable */

import { Contract, Signer, utils } from 'ethers'
import { Provider } from '@ethersproject/providers'
import type { ICustomGateway, ICustomGatewayInterface } from '../ICustomGateway'

const _abi = [
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        internalType: 'address',
        name: 'l1Address',
        type: 'address',
      },
      {
        indexed: true,
        internalType: 'address',
        name: 'l2Address',
        type: 'address',
      },
    ],
    name: 'TokenSet',
    type: 'event',
  },
  {
    inputs: [
      {
        internalType: 'address',
        name: '_l1Token',
        type: 'address',
      },
    ],
    name: 'l1ToL2Token',
    outputs: [
      {
        internalType: 'address',
        name: '_l2Token',
        type: 'address',
      },
    ],
    stateMutability: 'view',
    type: 'function',
  },
]

export class ICustomGateway__factory {
  static readonly abi = _abi
  static createInterface(): ICustomGatewayInterface {
    return new utils.Interface(_abi) as ICustomGatewayInterface
  }
  static connect(
    address: string,
    signerOrProvider: Signer | Provider
  ): ICustomGateway {
    return new Contract(address, _abi, signerOrProvider) as ICustomGateway
  }
}
